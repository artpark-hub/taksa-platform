package data

import (
	"context"
	"encoding/json"
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

// TableName overrides the default table name
func (LogORM) TableName() string { return "traceability_log" }

// UMHEvent defines the incoming JSON structure
type UMHEvent struct {
	EquipmentID   string `json:"equipment_id"`
	EventType     string `json:"event_type"`
	WorkOrderID   string `json:"work_order_id"`
	MaterialLotID string `json:"material_lot_id"`
	OperatorID    string `json:"operator_id"`
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

// Start subscribes to NATS and processes messages
func (c *Consumer) Start(ctx context.Context) error {
	js, err := c.data.nc.JetStream()
	if err != nil {
		return err
	}

	// The logic to run when a message arrives
	callback := func(msg *nats.Msg) {
		var event UMHEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			c.log.Errorf("JSON Parse Failed: %v", err)
			msg.Ack() // Ack bad data to stop loops
			return
		}

		if event.EquipmentID == "" {
			c.log.Warn("⚠️ Skipping event: Missing EquipmentID")
			msg.Ack()
			return
		}

		logEntry := LogORM{
			EquipmentID: event.EquipmentID,
			EventType:   event.EventType,
			WorkOrderID: event.WorkOrderID,
			OperatorID:  event.OperatorID,
			EventTime:   time.Now(),
		}

		// Save to Postgres
		if err := c.data.db.Create(&logEntry).Error; err != nil {
			c.log.Errorf("DB Insert Failed: %v", err)
			return // Do NOT Ack, let NATS retry
		}

		c.log.Infof("📥 Telemetry Saved: %s | %s", logEntry.EquipmentID, logEntry.EventType)
		msg.Ack()
	}

	// Subscribe to the Stream
	_, err = js.Subscribe("umh.>", callback, nats.BindStream("UMH_DATA"))
	if err != nil {
		return err
	}

	c.log.Info("🚀 Telemetry Service Started: Listening to UMH_DATA stream...")

	// Block here until the server stops
	<-ctx.Done()
	return nil
}
