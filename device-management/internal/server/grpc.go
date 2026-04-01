package server

import (
	devicemgmt "taksa-platform-dm/api/devicemgmt/v1"
	umhcore "taksa-platform-dm/api/umh-core/v2"
	"taksa-platform-dm/internal/conf"
	"taksa-platform-dm/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(
	c *conf.Server,
	instance *service.InstanceService,
	deviceMgmt *service.DeviceMgmtService,
	logger log.Logger,
) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	umhcore.RegisterInstanceServiceServer(srv, instance)
	devicemgmt.RegisterDeviceMgmtServiceServer(srv, deviceMgmt)
	return srv
}
