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
	data *Data
	log  *log.Helper
}

func NewConsumer(data *Data, logger log.Logger) *Consumer {
	return &Consumer{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	js, err := c.data.nc.JetStream()
	if err != nil {
		return err
	}

	callback := func(msg *nats.Msg) {
		parts := strings.Split(msg.Subject, ".")
		equipmentID := "UNKNOWN"
		if len(parts) > 0 {
			equipmentID = parts[len(parts)-1]
		}

		// 2. Parse the Raw JSON
		var rawEvent RawUMHEvent
		if err := json.Unmarshal(msg.Data, &rawEvent); err != nil {
			c.log.Errorf("JSON Parse Failed: %v", err)
			msg.Ack()
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
			if strings.Contains(v, "EncodingMask") {
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
			return
		}

		c.log.Infof("📥 Saved: %s | %s | %v", logEntry.EquipmentID, logEntry.EventType, rawEvent.Value)
		msg.Ack()
	}

	// Subscribe to everything under "umh.>"
	_, err = js.Subscribe("umh.>", callback, nats.BindStream("UMH_DATA"))
	if err != nil {

		c.log.Warnf("JetStream subscribe failed (%v), trying standard sub...", err)
		_, err = c.data.nc.Subscribe("umh.>", callback)
		if err != nil {
			return err
		}
	}

	c.log.Info("🚀 Telemetry Service Started: Listening to UMH real data...")
	<-ctx.Done()
	return nil
}
