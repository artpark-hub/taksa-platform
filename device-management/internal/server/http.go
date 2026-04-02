package server

import (
	"encoding/json"
	"net/http"

	devicemgmt "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	umhcore "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
	"github.com/artpark-hub/taksa-platform/device-management/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"go.uber.org/zap"
)

// loginResponseEncoder encodes LoginResponse with JWT token as HttpOnly cookie
// Only the cookie should contain the JWT token, NOT the response body
// This matches umh-core's security model
func loginResponseEncoder(w http.ResponseWriter, r *http.Request, v interface{}) error {
	if loginResp, ok := v.(*v2.LoginResponse); ok {
		// Set JWT cookie if token exists
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
			
			// Remove JWT token from response body (only send in cookie)
			// This matches umh-core's InstanceLoginResponse structure
			loginResp.JwtToken = ""
		}
	}
	
	// Use protojson with EmitDefaults to ensure empty fields are included
	// proto3 by default omits zero/empty values, which breaks empty list APIs
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	
	if protoMsg, ok := v.(proto.Message); ok {
		// Marshal with protojson to handle proto types properly
		data, err := protojson.Marshal(protoMsg)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		w.Write([]byte("\n"))
		return err
	}
	
	// Fallback to standard JSON encoding for non-proto types
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
			service.AuthMiddleware(zapLogger),
			service.SetLoginCookieMiddleware(zapLogger),
			service.ExtractJWTTokenMiddleware(zapLogger),
		),
		// Use custom encoder for Login responses
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
	umhcore.RegisterInstanceServiceHTTPServer(srv, instance)
	devicemgmt.RegisterDeviceMgmtServiceHTTPServer(srv, deviceMgmt)
	return srv
}

