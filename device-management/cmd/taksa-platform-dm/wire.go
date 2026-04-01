//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"taksa-platform-dm/internal/biz"
	"taksa-platform-dm/internal/conf"
	"taksa-platform-dm/internal/data"
	"taksa-platform-dm/internal/server"
	"taksa-platform-dm/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"go.uber.org/zap"
)

// wireApp init kratos application.
func wireApp(serverConf *conf.Server, dataConf *conf.Data, logger log.Logger, zapLogger *zap.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
