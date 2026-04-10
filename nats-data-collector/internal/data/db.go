package data

import (
	"fmt"
	"strings"

	"nats-data-collector/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewDB(c *conf.Data, logger log.Logger) (*gorm.DB, func(), error) {
	l := log.NewHelper(logger)
	var db *gorm.DB
	var err error

	switch strings.ToLower(c.Database.Driver) {
	case "", "postgres", "pg":
		db, err = gorm.Open(postgres.Open(c.Database.Source), &gorm.Config{})
		if err != nil {
			l.Errorf("failed opening connection to postgres: %v", err)
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("unsupported database driver: %s", c.Database.Driver)
	}

	l.Info("✅ Connected to Database")

	cleanup := func() {
		l.Info("closing database connection")
		sqlDB, err := db.DB()
		if err != nil {
			l.Errorf("failed to retrieve underlying sql DB: %v", err)
			return
		}
		if err := sqlDB.Close(); err != nil {
			l.Errorf("failed to close sql DB: %v", err)
		}
	}

	return db, cleanup, nil
}
