package server

import (
	"encoding/json"
	"net/http"

	devicemgmt "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"go.uber.org/zap"
)

// loginResponseEncoder handles Login response encoding with cookie management
func loginResponseEncoder(w http.ResponseWriter, r *http.Request, v interface{}) error {
	// Handle LoginResponse - set JWT cookie before writing body
	if loginResp, ok := v.(*v2.LoginResponse); ok {
		// Set cookie FIRST, before any writes
		if loginResp.JwtToken != "" {
			cookie := &http.Cookie{
				Name:     "token",
				Value:    loginResp.JwtToken,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				MaxAge:   3600,
			}
			http.SetCookie(w, cookie)
			// Remove JWT from response body
			loginResp.JwtToken = ""
		}
	}

	// Default Kratos proto/JSON encoding
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	
	if protoMsg, ok := v.(proto.Message); ok {
		data, err := protojson.Marshal(protoMsg)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	
	return json.NewEncoder(w).Encode(v)
}

// NewHTTPServer new an HTTP server.
func NewHTTPServer(
	c *conf.Server,
	instance *service.InstanceService,
	deviceMgmt *service.DeviceMgmtService,
	logger log.Logger,
	zapLogger *zap.Logger,
) *khttp.Server {
	var opts = []khttp.ServerOption{
		khttp.Middleware(
			recovery.Recovery(),
			middleware.HTTPTenantMiddleware(zapLogger, "your-secret-key-change-in-production"),
			service.AuthMiddleware(zapLogger),
			service.ExtractJWTTokenMiddleware(zapLogger),
		),
		khttp.ResponseEncoder(loginResponseEncoder),
	}
	if c.Http.Network != "" {
		opts = append(opts, khttp.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, khttp.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, khttp.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := khttp.NewServer(opts...)
	v2.RegisterInstanceServiceHTTPServer(srv, instance)
	devicemgmt.RegisterDeviceMgmtServiceHTTPServer(srv, deviceMgmt)
	return srv
}

