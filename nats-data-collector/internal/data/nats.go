package data

import (
	"nats-data-collector/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/nats-io/nats.go"
)

func NewNats(c *conf.Data, logger log.Logger) (*nats.Conn, func(), error) {
	l := log.NewHelper(logger)
	nc, err := nats.Connect(c.Nats.Url)
	if err != nil {
		l.Errorf("failed connecting to nats: %v", err)
		return nil, nil, err
	}
	l.Infof("✅ Connected to NATS at %s", c.Nats.Url)

	cleanup := func() {
		l.Info("closing nats connection")
		nc.Close()
	}

	return nc, cleanup, nil
}
