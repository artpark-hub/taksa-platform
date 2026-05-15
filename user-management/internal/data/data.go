package data

import (
	"context"
	"database/sql"
	"net/http"
	"time"
	"user-management/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	_ "github.com/lib/pq"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewTenantsRepo)

type Data struct {
	kratosAdminURL  string
	kratosPublicURL string
	httpClient      *http.Client
	db              *sql.DB
}

func (d *Data) KratosAdminURL() string {
	return d.kratosAdminURL
}

func (d *Data) KratosPublicURL() string {
	return d.kratosPublicURL
}

func (d *Data) HTTPClient() *http.Client {
	return d.httpClient
}

func (d *Data) DB() *sql.DB {
	return d.db
}

func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)

	var db *sql.DB
	if c.GetDatabase() != nil && c.GetDatabase().GetSource() != "" {
		driver := c.GetDatabase().GetDriver()
		if driver == "" {
			driver = "postgres"
		}

		var err error
		db, err = sql.Open(driver, c.GetDatabase().GetSource())
		if err != nil {
			return nil, nil, err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err = db.PingContext(ctx); err != nil {
			_ = db.Close()
			return nil, nil, err
		}

		helper.Info("database connection initialized; schema management is handled by deployment SQL")
	}

	cleanup := func() {
		if db != nil {
			if err := db.Close(); err != nil {
				helper.Errorf("failed to close database connection: %v", err)
			}
		}
		helper.Info("closing the data resources")
	}

	return &Data{
		kratosAdminURL:  c.KratosAdminUrl,
		kratosPublicURL: c.KratosPublicUrl,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		db: db,
	}, cleanup, nil
}
