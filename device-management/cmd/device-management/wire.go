//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/artpark-hub/taksa-platform/device-management/internal/biz"
	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
	"github.com/artpark-hub/taksa-platform/device-management/internal/data"
	"github.com/artpark-hub/taksa-platform/device-management/internal/server"
	"github.com/artpark-hub/taksa-platform/device-management/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"go.uber.org/zap"
)

// wireApp init kratos application.
func wireApp(serverConf *conf.Server, dataConf *conf.Data, deploymentConf *conf.Deployment, logger log.Logger, zapLogger *zap.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
