//go:build wireinject
// +build wireinject

package main

import (
	"user-management/internal/biz"
	"user-management/internal/conf"
	"user-management/internal/data"
	"user-management/internal/server"
	"user-management/internal/service"

	_ "github.com/fsnotify/fsnotify"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
// FIXED: Added *conf.Kratos to the parameters list
func wireApp(*conf.Server, *conf.Data, *conf.Kratos, *conf.Nats, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
