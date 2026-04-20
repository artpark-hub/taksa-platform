package middleware

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	httptransport "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// isPublicPath returns true for endpoints that don't require JWT authentication.
// - /health: service health check
// - /instance/login: device login (authenticates via auth token, not JWT)
func isPublicPath(urlPath string) bool {
	// IMPORTANT:
	// - Only match against the parsed URL path (no query string).
	// - Do NOT use substring matching (strings.Contains), as that can accidentally
	//   exempt protected endpoints like "/api/v2/instance/loginSomething".
	//
	// Normalize a trailing slash so "/health/" behaves like "/health".
	path := strings.TrimRight(urlPath, "/")
	if path == "" {
		path = "/"
	}

	switch path {
	case "/health":
		return true
	case "/api/v2/instance/login":
		return true
	default:
		return false
	}
}

// HTTPTenantMiddleware is a Kratos HTTP middleware that extracts tenant_id and device_id
// from the JWT and stores them in context.
//
// Two JWT sources with different trust models:
//   - Authorization header: JWT validated upstream by Oathkeeper → ParseUnverified (trusted proxy)
//   - "token" cookie: JWT issued by this service during Login → signature verified with jwtSecret
//
// Public paths (health, login) are exempt. All other paths MUST have a valid JWT
// with a tenant_id claim or the request is rejected.
func HTTPTenantMiddleware(logger *zap.Logger, jwtSecret string) middleware.Middleware {
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

			// Public paths don't require JWT
			if isPublicPath(request.URL.Path) {
				return handler(ctx, req)
			}

			// Try Authorization header first (console APIs via Oathkeeper)
			tokenStr := ""
			fromCookie := false
			if authHeader := request.Header.Get("Authorization"); authHeader != "" {
				tokenStr = extractBearerTokenFromHeader(authHeader)
			}
			// Fall back to "token" cookie (device APIs after Login)
			if tokenStr == "" {
				if cookie, err := request.Cookie("token"); err == nil && cookie.Value != "" {
					tokenStr = cookie.Value
					fromCookie = true
				}
			}

			if tokenStr == "" {
				logger.Warn("Rejected: no JWT in Authorization header or cookie",
					zap.String("path", request.RequestURI))
				return nil, status.Error(codes.Unauthenticated, "missing authorization token")
			}

			// Parse JWT claims with appropriate trust level
			var claims jwt.MapClaims
			if fromCookie {
				// Cookie JWT was issued by us — verify signature
				claims = jwt.MapClaims{}
				token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
					if token.Method != jwt.SigningMethodHS256 {
						return nil, jwt.ErrTokenSignatureInvalid
					}
					return []byte(jwtSecret), nil
				})
				if err != nil || !token.Valid {
					logger.Warn("Rejected: invalid cookie JWT signature",
						zap.Error(err), zap.String("path", request.RequestURI))
					return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
				}
			} else {
				// Authorization header JWT validated by Oathkeeper upstream — trust it
				claims = jwt.MapClaims{}
				_, _, err := new(jwt.Parser).ParseUnverified(tokenStr, claims)
				if err != nil {
					logger.Warn("Rejected: invalid JWT", zap.Error(err),
						zap.String("path", request.RequestURI))
					return nil, status.Error(codes.Unauthenticated, "invalid authorization token")
				}
			}

			// Extract tenant_id — required for all protected paths
			tenantID, ok := claims["tenant_id"].(string)
			if !ok || tenantID == "" {
				logger.Warn("Rejected: missing tenant_id in JWT claims",
					zap.String("path", request.RequestURI))
				return nil, status.Error(codes.PermissionDenied, "missing tenant_id in token")
			}
			ctx = context.WithValue(ctx, TenantIDContextKey, tenantID)

			// Extract device_id (present in device JWTs, absent in console JWTs)
			if deviceID, ok := claims["device_id"].(string); ok && deviceID != "" {
				ctx = context.WithValue(ctx, DeviceIDContextKey, deviceID)
			}

			return handler(ctx, req)
		}
	}
}

func extractBearerTokenFromHeader(authHeader string) string {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != BearerScheme {
		return ""
	}
	return parts[1]
}
