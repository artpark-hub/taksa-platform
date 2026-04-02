package data

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm/clause"
)

type EquipmentMasterORM struct {
	ID                string `gorm:"primaryKey;column:id"`
	OperationalStatus string `gorm:"column:operational_status"`
}

func (EquipmentMasterORM) TableName() string { return "equipment_master" }

type TelemetryORM struct {
	ID            int32     `gorm:"primaryKey;autoIncrement"`
	EquipmentID   string    `gorm:"column:equipment_id"`
	ParameterName string    `gorm:"column:parameter_name"`
	Value         float64   `gorm:"column:value"`
	UnitOfMeasure string    `gorm:"column:unit_of_measure"`
	RecordedAt    time.Time `gorm:"column:recorded_at"`
}

func (TelemetryORM) TableName() string { return "equipment_telemetry" }

type RawUNSEvent struct {
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
	if !c.subscribed {
		if err := c.Subscribe(); err != nil {
			return err
		}
	}

	c.log.Info("🚀 Telemetry Service Started: Listening to UNS real data...")
	<-ctx.Done()
	return nil
}

func (c *Consumer) handleMsg(msg *nats.Msg) {
	parts := strings.Split(msg.Subject, ".")
	equipmentID := "UNKNOWN"
	parameterName := "UNKNOWN"
	switch {
	case len(parts) >= 2:
		equipmentID = parts[len(parts)-2]
		parameterName = parts[len(parts)-1]
	case len(parts) == 1:
		equipmentID = parts[0]
	}

	var rawEvent RawUNSEvent
	if err := json.Unmarshal(msg.Data, &rawEvent); err != nil {
		c.log.Errorf("JSON parse failed for subject=%s: %v", msg.Subject, err)
		c.publishToDLQ(msg, err.Error())
		return
	}

	numericValue, ok := rawEvent.Value.(float64)
	if !ok {
		c.log.Warnf("non-numeric value on subject=%s (type=%T value=%v) — skipping telemetry insert, routing to DLQ",
			msg.Subject, rawEvent.Value, rawEvent.Value)
		c.publishToDLQ(msg, "non-numeric value cannot be stored in equipment_telemetry")
		return
	}

	if err := c.ensureEquipment(equipmentID); err != nil {
		c.log.Errorf("failed to auto-register equipment=%s: %v — dropping message", equipmentID, err)
		return
	}

	entry := TelemetryORM{
		EquipmentID:   equipmentID,
		ParameterName: parameterName,
		Value:         numericValue,
		RecordedAt:    time.UnixMilli(rawEvent.TimestampMS),
	}

	if err := c.data.db.Create(&entry).Error; err != nil {
		c.log.Errorf("DB insert failed for equipment=%s param=%s: %v", equipmentID, parameterName, err)
		c.publishToDLQ(msg, "database error: failed to insert telemetry into equipment_telemetry")
		return
	}

	c.log.Infof("📥 Saved: equipment=%s param=%s value=%v recorded_at=%s",
		entry.EquipmentID, entry.ParameterName, entry.Value, entry.RecordedAt.Format(time.RFC3339))

	if _, merr := msg.Metadata(); merr == nil {
		if err := msg.Ack(); err != nil {
			c.log.Errorf("failed to ack JetStream message: %v", err)
		}
	}
}

func (c *Consumer) ensureEquipment(equipmentID string) error {
	stub := EquipmentMasterORM{
		ID:                equipmentID,
		OperationalStatus: "active",
	}
	result := c.data.db.
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&stub)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		c.log.Infof("🆕 Auto-registered new equipment: %s", equipmentID)
	}
	return nil
}

func (c *Consumer) publishToDLQ(msg *nats.Msg, reason string) {
	dlq := c.data.dlqSubject
	if dlq == "" {
		c.log.Errorf("no DLQ configured — dropping message: subject=%s reason=%s payload=%s",
			msg.Subject, reason, string(msg.Data))
		return
	}

	meta := map[string]string{"source_subject": msg.Subject, "error": reason}
	metaB, err := json.Marshal(meta)
	if err != nil {
		c.log.Errorf("failed to marshal DLQ metadata for subject %s: %v", msg.Subject, err)
	}
	payload := append(metaB, '\n')
	payload = append(payload, msg.Data...)

	if err := c.data.nc.Publish(dlq, payload); err != nil {
		c.log.Errorf("failed to publish to DLQ %s: %v", dlq, err)
	} else {
		c.log.Infof("forwarded problematic message to DLQ %s (reason: %s)", dlq, reason)
	}

	if _, merr := msg.Metadata(); merr == nil {
		if err := msg.Ack(); err != nil {
			c.log.Errorf("failed to ack message after DLQ publish: %v", err)
		}
	}
}

func (c *Consumer) Subscribe() error {
	if c.subscribed {
		return nil
	}

	js, err := c.data.nc.JetStream()
	if err != nil {
		return err
	}

	subject := c.data.subject
	if subject == "" {
		subject = "uns.v1.>"
	}
	queueGroup := c.data.queueGroup
	stream := c.data.streamName
	if stream == "" {
		stream = "UNS_DATA"
	}

	if queueGroup != "" {
		_, err = js.QueueSubscribe(subject, queueGroup, c.handleMsg, nats.BindStream(stream))
	} else {
		_, err = js.Subscribe(subject, c.handleMsg, nats.BindStream(stream))
	}
	if err != nil {
		c.log.Warnf("JetStream subscribe failed (%v), evaluating fallback...", err)
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "auth") || strings.Contains(low, "permission") || strings.Contains(low, "authorization") {
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
