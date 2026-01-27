package data

import (
	"telemetry-service/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/nats-io/nats.go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ProviderSet is used by Wire to inject dependencies
var ProviderSet = wire.NewSet(NewData, NewConsumer)

type Data struct {
	db *gorm.DB
	nc *nats.Conn
}

// NewData connects to Postgres and NATS
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	l := log.NewHelper(logger)

	// 1. Connect to DB
	db, err := gorm.Open(postgres.Open(c.Database.Source), &gorm.Config{})
	if err != nil {
		l.Errorf("failed opening connection to postgres: %v", err)
		return nil, nil, err
	}
	l.Info("✅ Connected to Database")

	// 2. Connect to NATS
	nc, err := nats.Connect(c.Nats.Url)
	if err != nil {
		l.Errorf("failed connecting to nats: %v", err)
		return nil, nil, err
	}
	l.Infof("✅ Connected to NATS at %s", c.Nats.Url)

	d := &Data{
		db: db,
		nc: nc,
	}

	cleanup := func() {
		l.Info("closing data resources")
		nc.Close()
	}

	return d, cleanup, nil
}
