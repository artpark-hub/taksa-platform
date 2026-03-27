package data

import (
	"net/http"
	"time"
	"user-management/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewTenantsRepo)

type Data struct {
	kratosAdminURL  string
	kratosPublicURL string
	httpClient      *http.Client
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

func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}

	return &Data{
		kratosAdminURL:  c.KratosAdminUrl,
		kratosPublicURL: c.KratosPublicUrl,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, cleanup, nil
}
