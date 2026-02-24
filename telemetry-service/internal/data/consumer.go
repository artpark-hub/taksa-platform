package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/nats-io/nats.go"
)

// LogORM defines the database schema
type LogORM struct {
	ID            int32 `gorm:"primaryKey;autoIncrement"`
	EquipmentID   string
	EventType     string
	WorkOrderID   string
	MaterialLotID string
	OperatorID    string
	EventTime     time.Time
}

func (LogORM) TableName() string { return "traceability_log" }

// RawUMHEvent captures the raw UMH structure
type RawUMHEvent struct {
	TimestampMS int64       `json:"timestamp_ms"`
	Value       interface{} `json:"value"`
}

type Consumer struct {
	data       *Data
	log        *log.Helper
	subscribed bool
}

func NewConsumer(data *Data, logger log.Logger) *Consumer {
	return &Consumer{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	// Ensure there's a subscription; if Subscribe() has already been called elsewhere,
	// don't attempt to subscribe again.
	if !c.subscribed {
		if err := c.Subscribe(); err != nil {
			return err
		}
	}

	c.log.Info("🚀 Telemetry Service Started: Listening to UMH real data...")
	<-ctx.Done()
	return nil
}

const (
	// EncodingMaskKeyword is used to detect complex UMH state blobs.
	// If the UMH protocol changes this marker, update this constant and document the format.
	EncodingMaskKeyword = "EncodingMask"
)

// handleMsg processes a single NATS message.
func (c *Consumer) handleMsg(msg *nats.Msg) {
	parts := strings.Split(msg.Subject, ".")
	equipmentID := "UNKNOWN"
	if len(parts) > 0 {
		equipmentID = parts[len(parts)-1]
	}

	// 2. Parse the Raw JSON
	var rawEvent RawUMHEvent
	if err := json.Unmarshal(msg.Data, &rawEvent); err != nil {
		c.log.Errorf("JSON Parse Failed: %v", err)
		// If DLQ is configured, publish the raw payload with an annotation
		// so operators can inspect and troubleshoot malformed messages.
		dlq := c.data.dlqSubject
		if dlq != "" {
			// Include original subject and error in the DLQ payload header
			meta := map[string]string{"source_subject": msg.Subject, "error": err.Error()}
			metaB, _ := json.Marshal(meta)
			payload := append(metaB, '\n')
			payload = append(payload, msg.Data...)
			if perr := c.data.nc.Publish(dlq, payload); perr != nil {
				c.log.Errorf("failed to publish to DLQ %s: %v", dlq, perr)
			} else {
				c.log.Infof("published malformed message to DLQ %s", dlq)
			}
			// For JetStream messages, ACK after publishing to DLQ to avoid redelivery
			if _, merr := msg.Metadata(); merr == nil {
				if err := msg.Ack(); err != nil {
					c.log.Errorf("failed to ack message after DLQ publish: %v", err)
				}
			}
			return
		}

		// No DLQ configured: log raw payload for inspection and do NOT ACK so
		// JetStream can retry; if this is plain NATS the message will be lost,
		// so we log the payload to help debugging.
		c.log.Errorf("malformed message (no DLQ configured): subject=%s payload=%s", msg.Subject, string(msg.Data))
		return
	}

	// 3. Convert Timestamp
	eventTime := time.UnixMilli(rawEvent.TimestampMS)

	// 4. Determine Event Type based on Value
	eventType := "STATUS_UPDATE"

	// Use type assertion to check what 'Value' is
	switch v := rawEvent.Value.(type) {
	case float64:
		eventType = fmt.Sprintf("METRIC_VALUE: %.0f", v)
	case string:
		if strings.Contains(v, EncodingMaskKeyword) {
			eventType = "COMPLEX_STATE"
		} else {
			eventType = "TIMESTAMP_UPDATE"
		}
	}

	// 5. Construct DB Object
	logEntry := LogORM{
		EquipmentID: equipmentID,
		EventType:   eventType,
		WorkOrderID: "WO-AUTO",
		OperatorID:  "OP-SYS",
		EventTime:   eventTime,
	}

	// 6. Save to Postgres
	if err := c.data.db.Create(&logEntry).Error; err != nil {
		c.log.Errorf("DB Insert Failed: %v", err)
		// For JetStream messages, avoid ACK so it can be retried; for plain NATS just log and return
		return
	}

	c.log.Infof("📥 Saved: %s | %s | %v", logEntry.EquipmentID, logEntry.EventType, rawEvent.Value)

	if _, merr := msg.Metadata(); merr == nil {
		if err := msg.Ack(); err != nil {
			c.log.Errorf("failed to ack message: %v", err)
		}
	}
}

// Subscribe creates the subscription(s) according to configuration. It is safe to call
// multiple times; subsequent calls are no-ops after a successful subscribe.
func (c *Consumer) Subscribe() error {
	if c.subscribed {
		return nil
	}

	js, err := c.data.nc.JetStream()
	if err != nil {
		return err
	}

	// Determine subject, queue group, and stream from configuration (fall back to hardcoded)
	subject := c.data.subject
	if subject == "" {
		subject = "umh.>"
	}
	queueGroup := c.data.queueGroup
	stream := c.data.streamName
	if stream == "" {
		stream = "UMH_DATA"
	}

	// Subscribe according to configured subject/queue; bind to configured stream when provided
	if queueGroup != "" {
		_, err = js.QueueSubscribe(subject, queueGroup, c.handleMsg, nats.BindStream(stream))
	} else {
		_, err = js.Subscribe(subject, c.handleMsg, nats.BindStream(stream))
	}
	if err != nil {
		// Log error details; if this looks like an auth/permission issue, surface it
		c.log.Warnf("JetStream subscribe failed (%v), evaluating fallback...", err)
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "auth") || strings.Contains(low, "permission") || strings.Contains(low, "authorization") {
			// Likely an auth/perm issue — don't silently fall back to plain NATS
			return err
		}

		c.log.Warnf("Falling back to plain NATS subscription for subject %s", subject)
		if queueGroup != "" {
			_, err = c.data.nc.QueueSubscribe(subject, queueGroup, c.handleMsg)
		} else {
			_, err = c.data.nc.Subscribe(subject, c.handleMsg)
		}
		if err != nil {
			return err
		}
	}

	c.subscribed = true
	return nil
}
