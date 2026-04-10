//go:build wireinject
// +build wireinject

package main

import (
	"nats-data-collector/internal/conf"
	"nats-data-collector/internal/data"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

func initApp(*conf.Server, *conf.Data, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(data.ProviderSet, newApp))
}
