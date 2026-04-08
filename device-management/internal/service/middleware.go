package service

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	httptransport "github.com/go-kratos/kratos/v2/transport/http"
	"go.uber.org/zap"
)

// Context key types for storing values in context
type ContextKey string

const (
	AuthorizationKey ContextKey = "authorization"
	JWTTokenKey      ContextKey = "jwt_token"
)

// AuthMiddleware extracts Bearer token from Authorization header and logs request details
func AuthMiddleware(logger *zap.Logger) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Get transport info to access headers
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}

			// Check if this is HTTP transport
			httpTr, ok := tr.(*httptransport.Transport)
			if !ok {
				return handler(ctx, req)
			}

			// Get request details
			request := httpTr.Request()
			logger.Debug("Incoming HTTP request",
				zap.String("method", request.Method),
				zap.String("path", request.RequestURI),
				zap.String("remote_addr", request.RemoteAddr),
			)

			// Get Authorization header
			authHeader := request.Header.Get("Authorization")

			// Extract Bearer token
			if authHeader != "" {
				const bearerPrefix = "Bearer "
				if strings.HasPrefix(authHeader, bearerPrefix) {
					token := strings.TrimPrefix(authHeader, bearerPrefix)
					logger.Debug("Authorization header found",
						zap.Bool("has_token", true),
					)
					// Store in context with typed key
					ctx = context.WithValue(ctx, AuthorizationKey, token)
				}
			} else {
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

// ExtractJWTTokenMiddleware extracts JWT token from the "token" cookie and stores it in context
// This allows downstream handlers (Push, Pull, etc.) to access the JWT for device context extraction
func ExtractJWTTokenMiddleware(logger *zap.Logger) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Get transport info to access request
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}

			// Check if this is HTTP transport
			httpTr, ok := tr.(*httptransport.Transport)
			if !ok {
				return handler(ctx, req)
			}

			// Get request to access cookies
			request := httpTr.Request()

			// Try to get JWT token from cookie
			cookie, err := request.Cookie("token")
			if err == nil && cookie.Value != "" {
				// Store JWT in context for use by services (with typed key)
				ctx = context.WithValue(ctx, JWTTokenKey, cookie.Value)
				logger.Debug("Extracted JWT token from cookie")
			} else {
				logger.Debug("No 'token' cookie found in request",
					zap.Error(err),
				)
			}

			return handler(ctx, req)
		}
	}
}
