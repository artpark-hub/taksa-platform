package data

import (
	"database/sql"
	"net/http"
	"user-management/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/nats-io/nats.go"

	// Import the Kratos client library
	kratos "github.com/ory/kratos-client-go"

	_ "github.com/lib/pq"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewGreeterRepo,
	NewTenantsRepo,
)

// Data holds database and external service connections
type Data struct {
	db              *sql.DB
	kratosAdminURL  string
	kratosPublicURL string
	httpClient      *http.Client
	kratos          *kratos.APIClient
	nats            *nats.Conn
	log             *log.Helper
}

func NewData(c *conf.Data, kConf *conf.Kratos, nConf *conf.Nats, logger log.Logger) (*Data, func(), error) {
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

	helper.Infof("Connecting to NATS at %s...", nConf.Addr)
	nc, err := nats.Connect(
		nConf.Addr,
		nats.UserInfo(nConf.User, nConf.Password),
		nats.MaxReconnects(5),
	)
	if err != nil {
		helper.Errorf("Failed to connect to NATS: %v", err)
		return nil, nil, err
	}
	helper.Info("NATS connected successfully")

	data := &Data{
		db:              db,
		kratosAdminURL:  kConf.AdminUrl,
		kratosPublicURL: kConf.PublicUrl,
		httpClient:      &http.Client{},
		kratos:          kratosClient,
		nats:            nc, // Store connection
		log:             helper,
	}

	cleanup := func() {
		helper.Info("Closing data resources")
		if err := db.Close(); err != nil {
			helper.Errorf("Failed to close database: %v", err)
		}
		// Close NATS
		nc.Drain()
		nc.Close()
		helper.Info("NATS connection closed")
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

func (d *Data) NatsClient() *nats.Conn {
	return d.nats
}
