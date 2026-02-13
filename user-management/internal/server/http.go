package server

import (
	tenantsv1 "user-management/api/tenants/v1"
	"user-management/internal/conf"
	"user-management/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new HTTP server.
// Notice we REMOVED GreeterService and ADDED TenantsService
func NewHTTPServer(c *conf.Server, tenants *service.TenantsService, logger log.Logger) *kratoshttp.Server {
	var opts = []kratoshttp.ServerOption{
		kratoshttp.Middleware(
			recovery.Recovery(),
		),
		// kratoshttp.Filter(corsMiddleware), // Add the custom CORS middleware here
	}
	if c.Http.Network != "" {
		opts = append(opts, kratoshttp.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, kratoshttp.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, kratoshttp.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := kratoshttp.NewServer(opts...)

	// Register the Tenants Service
	tenantsv1.RegisterTenantsServiceHTTPServer(srv, tenants)

	return srv
}

// func corsMiddleware(handler http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.Header().Set("Access-Control-Allow-Origin", "*")
// 		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
// 		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

// 		if r.Method == "OPTIONS" {
// 			w.WriteHeader(http.StatusNoContent)
// 			return
// 		}

// 		handler.ServeHTTP(w, r)
// 	})
// }
