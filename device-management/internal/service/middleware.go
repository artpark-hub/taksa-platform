package service

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	httptransport "github.com/go-kratos/kratos/v2/transport/http"
	"go.uber.org/zap"

	mw "github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
)

// Context key types for storing values in context
type ContextKey string

const (
	AuthorizationKey ContextKey = "authorization"
	JWTTokenKey      ContextKey = "jwt_token"
)

// AuthMiddleware extracts Bearer token from Authorization header into context for Login.
// Public paths (/health, login) skip noisy debug logs; login still requires Bearer extraction.
func AuthMiddleware(logger *zap.Logger) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}

			httpTr, ok := tr.(*httptransport.Transport)
			if !ok {
				return handler(ctx, req)
			}

			request := httpTr.Request()
			public := mw.IsPublicPath(request.URL.Path)

			if !public {
				logger.Debug("Incoming HTTP request",
					zap.String("method", request.Method),
					zap.String("path", request.RequestURI),
					zap.String("remote_addr", request.RemoteAddr),
				)
			}

			authHeader := request.Header.Get("Authorization")
			if authHeader != "" {
				const bearerPrefix = "Bearer "
				if strings.HasPrefix(authHeader, bearerPrefix) {
					token := strings.TrimPrefix(authHeader, bearerPrefix)
					if !public {
						logger.Debug("Authorization header found",
							zap.Bool("has_token", true),
						)
					}
					ctx = mw.SetAuthorizationToken(ctx, token)
				}
			} else if !public {
				logger.Debug("No authorization header provided",
					zap.String("path", request.RequestURI),
				)
			}

			return handler(ctx, req)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SetLoginCookieMiddleware is no-op - cookie is now handled in loginResponseEncoder
func SetLoginCookieMiddleware(logger *zap.Logger) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			return handler(ctx, req)
		}
	}
}

// ExtractJWTTokenMiddleware extracts JWT token from the "token" cookie and stores it in context.
// Public paths skip cookie debug logs (health probes, login before cookie is set).
func ExtractJWTTokenMiddleware(logger *zap.Logger) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}

			httpTr, ok := tr.(*httptransport.Transport)
			if !ok {
				return handler(ctx, req)
			}

			request := httpTr.Request()
			if mw.IsPublicPath(request.URL.Path) {
				return handler(ctx, req)
			}

			cookie, err := request.Cookie("token")
			if err == nil && cookie.Value != "" {
				ctx = context.WithValue(ctx, JWTTokenKey, cookie.Value)
				logger.Debug("Extracted JWT token from cookie")
			} else if err != nil {
				logger.Debug("No 'token' cookie found in request",
					zap.String("path", request.URL.Path),
				)
			}

			return handler(ctx, req)
		}
	}
}
