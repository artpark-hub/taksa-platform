package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	httptransport "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

const (
	cookieJWTLeeway                  = 2 * time.Minute
	authRealm                        = "device-management"
	reasonMissingAuthorizationToken  = "missing_authorization_token"
	reasonInvalidToken               = "invalid_token"
	reasonTokenExpired               = "token_expired"
	reasonDeviceJWTSignatureRequired = "device_jwt_signature_required"
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

// requiresVerifiedDeviceSessionHTTP is true for edge Instance routes that must not use
// unverified Authorization bearer claims alone (cookie or DM-signed HS256 bearer required).
func requiresVerifiedDeviceSessionHTTP(urlPath string) bool {
	p := strings.TrimRight(urlPath, "/")
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(p, "/api/v2/instance/") {
		return false
	}
	return p != "/api/v2/instance/login"
}

// HTTPTenantMiddleware is a Kratos HTTP middleware that extracts tenant_id and device_id
// from the JWT and stores them in context.
//
// Two JWT sources with different trust models:
//   - Authorization header: JWT parsed for claims (often validated upstream by Oathkeeper). If the
//     bearer token is an HS256 JWT signed with jwtSecret (same as Login), it is verified here.
//   - "token" cookie: JWT issued by this service during Login → signature verified with jwtSecret
//
// Edge Instance HTTP paths (/api/v2/instance/* except login) require a signature-verified device JWT
// (cookie or verified HS256 bearer), not unverified bearer claims alone.
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
				return nil, unauthorizedHTTP(httpTr, reasonMissingAuthorizationToken, "missing authorization token")
			}

			// Parse JWT claims with appropriate trust level
			var claims jwt.MapClaims
			if fromCookie {
				// Cookie JWT was issued by us — verify signature
				claims = jwt.MapClaims{}
				parser := jwt.NewParser(
					jwt.WithLeeway(cookieJWTLeeway),
					jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
				)
				token, err := parser.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
					return []byte(jwtSecret), nil
				})
				if err != nil {
					reason := reasonInvalidToken
					message := "invalid token"
					logMessage := "Rejected: invalid cookie JWT"
					if errors.Is(err, jwt.ErrTokenExpired) {
						reason = reasonTokenExpired
						message = "token expired"
						logMessage = "Rejected: expired cookie JWT"
					}
					logger.Warn(logMessage,
						zap.Error(err), zap.String("path", request.RequestURI))
					return nil, unauthorizedHTTP(httpTr, reason, message)
				}
				if !token.Valid {
					logger.Warn("Rejected: invalid cookie JWT",
						zap.String("path", request.RequestURI))
					return nil, unauthorizedHTTP(httpTr, reasonInvalidToken, "invalid token")
				}
			} else {
				// Authorization header JWT validated by Oathkeeper upstream — trust it
				claims = jwt.MapClaims{}
				_, _, err := new(jwt.Parser).ParseUnverified(tokenStr, claims)
				if err != nil {
					logger.Warn("Rejected: invalid JWT", zap.Error(err),
						zap.String("path", request.RequestURI))
					return nil, unauthorizedHTTP(httpTr, reasonInvalidToken, "invalid authorization token")
				}
			}

			// Extract tenant_id — required for all protected paths
			tenantID, ok := claims["tenant_id"].(string)
			if !ok || tenantID == "" {
				logger.Warn("Rejected: missing tenant_id in JWT claims",
					zap.String("path", request.RequestURI))
				return nil, unauthorizedHTTP(httpTr, reasonInvalidToken, "missing tenant_id in token")
			}
			ctx = context.WithValue(ctx, TenantIDContextKey, tenantID)

			// Extract device_id (present in device JWTs, absent in console JWTs)
			if deviceID, ok := claims["device_id"].(string); ok && deviceID != "" {
				ctx = context.WithValue(ctx, DeviceIDContextKey, deviceID)
			}

			// Mark when this service has verified the JWT signature (cookie or HS256 bearer from Login).
			if fromCookie {
				ctx = MarkDeviceJWTSignatureVerified(ctx)
			} else if jwtSecret != "" {
				if _, _, ok := tryVerifyDeviceJWTHS256(tokenStr, jwtSecret); ok {
					ctx = MarkDeviceJWTSignatureVerified(ctx)
				}
			}

			if requiresVerifiedDeviceSessionHTTP(request.URL.Path) && !IsDeviceJWTSignatureVerified(ctx) {
				logger.Warn("Rejected: instance API requires verified device JWT (token cookie or Login-issued HS256 bearer)",
					zap.String("path", request.RequestURI))
				return nil, unauthorizedHTTP(httpTr, reasonDeviceJWTSignatureRequired,
					"use token cookie or Authorization bearer JWT signed by device-management (Login)")
			}

			return handler(ctx, req)
		}
	}
}

func unauthorizedHTTP(httpTr *httptransport.Transport, reason, message string) error {
	httpTr.ReplyHeader().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q, error=%q, error_description=%q`, authRealm, reason, message))
	return kerrors.Unauthorized(reason, message)
}

func extractBearerTokenFromHeader(authHeader string) string {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != BearerScheme {
		return ""
	}
	return parts[1]
}
