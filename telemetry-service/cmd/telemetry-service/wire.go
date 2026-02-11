//go:build wireinject
// +build wireinject

package main

import (
	"telemetry-service/internal/conf"
	"telemetry-service/internal/data"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// initApp init kratos application.
func initApp(*conf.Server, *conf.Data, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(data.ProviderSet, newApp))
}
