package data

import (
	"database/sql"
	"net/http"
	"user-management/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"

	// Import the Kratos client library
	kratos "github.com/ory/kratos-client-go"

	_ "github.com/lib/pq"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewTenantsRepo,
)

// Data holds database and external service connections
type Data struct {
	db              *sql.DB
	kratosAdminURL  string
	kratosPublicURL string
	httpClient      *http.Client
	kratos          *kratos.APIClient
	log             *log.Helper
}

func NewData(c *conf.Data, kConf *conf.Kratos, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)

	db, err := sql.Open(c.Database.Driver, c.Database.Source)
	if err != nil {
		return nil, nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, nil, err
	}
	helper.Info("Database connected successfully")

	kratosConfig := kratos.NewConfiguration()
	kratosConfig.Servers = kratos.ServerConfigurations{
		{
			URL: kConf.AdminUrl,
		},
	}
	kratosClient := kratos.NewAPIClient(kratosConfig)

	// REMOVED: NATS Connection Block

	data := &Data{
		db:              db,
		kratosAdminURL:  kConf.AdminUrl,
		kratosPublicURL: kConf.PublicUrl,
		httpClient:      &http.Client{},
		kratos:          kratosClient,
		log:             helper,
		// REMOVED: nats: nc,
	}

	cleanup := func() {
		helper.Info("Closing data resources")
		if err := db.Close(); err != nil {
			helper.Errorf("Failed to close database: %v", err)
		}
		// REMOVED: NATS cleanup
	}

	return data, cleanup, nil
}

// --- Getter Methods ---

func (d *Data) DB() *sql.DB {
	return d.db
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

func (d *Data) KratosClient() *kratos.APIClient {
	return d.kratos
}
