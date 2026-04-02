package data

import (
	"nats-data-collector/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
)

var ProviderSet = wire.NewSet(NewData, NewConsumer)

type Data struct {
	db         *gorm.DB
	nc         *nats.Conn
	subject    string
	queueGroup string
	streamName string
	dlqSubject string
}

func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	db, dbCleanup, err := NewDB(c, logger)
	if err != nil {
		return nil, nil, err
	}

	nc, natsCleanup, err := NewNats(c, logger)
	if err != nil {
		dbCleanup()
		return nil, nil, err
	}

	d := &Data{
		db:         db,
		nc:         nc,
		subject:    c.Nats.GetSubject(),
		queueGroup: c.Nats.GetQueueGroup(),
		streamName: c.Nats.GetStream(),
		dlqSubject: c.Nats.GetDlq(),
	}

	cleanup := func() {
		log.NewHelper(logger).Info("closing data resources")
		natsCleanup()
		dbCleanup()
	}

	return d, cleanup, nil
}
