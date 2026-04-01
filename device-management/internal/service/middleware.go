package service

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	httptransport "github.com/go-kratos/kratos/v2/transport/http"
	"go.uber.org/zap"
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
						zap.String("token_preview", token[:min(len(token), 20)]+"..."),
					)
					// Store in context
					ctx = context.WithValue(ctx, "authorization", token)
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

// SetLoginCookieMiddleware is deprecated - cookie is now set in loginResponseEncoder
// Kept as no-op for compatibility
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

			// logger.Debug("Cookies in request",
			// 	zap.String("cookie_header", request.Header.Get("Cookie")),
			// )

			// Try to get JWT token from cookie
			cookie, err := request.Cookie("token")
			if err == nil && cookie.Value != "" {
				// Store JWT in context for use by services
				ctx = context.WithValue(ctx, "jwt_token", cookie.Value)
				logger.Debug("Extracted JWT token from cookie",
					zap.String("token_preview", cookie.Value[:min(len(cookie.Value), 20)]+"..."),
				)
			} else {
				logger.Debug("No 'token' cookie found in request",
					zap.Error(err),
				)
			}

			return handler(ctx, req)
		}
	}
}
