package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	AuthorizationHeader = "authorization"
	TenantIDContextKey  = "tenant_id"
	DeviceIDContextKey  = "device_id"
	BearerScheme        = "Bearer"
)

// ExtractClaimsFromJWT extracts tenant_id and device_id from JWT claims in Authorization header
// Assumes JWT has been pre-validated by upstream auth gateway (Oathkeeper)
// Returns both tenant_id and device_id from claims without re-validating signature
func ExtractClaimsFromJWT(ctx context.Context) (tenantID, deviceID string, err error) {
	// Get metadata from context
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Get Authorization header
	authHeaders := md.Get(AuthorizationHeader)
	if len(authHeaders) == 0 {
		return "", "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// Parse Bearer token
	token := extractBearerToken(authHeaders[0])
	if token == "" {
		return "", "", status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	// Parse JWT claims without signature verification
	// Safe because Oathkeeper/HAProxy already validated the signature
	claims := jwt.MapClaims{}
	_, _, err = new(jwt.Parser).ParseUnverified(token, claims)
	if err != nil {
		return "", "", status.Error(codes.Unauthenticated, fmt.Sprintf("failed to parse JWT: %v", err))
	}

	// Extract tenant_id claim
	tenantID, ok = claims["tenant_id"].(string)
	if !ok || tenantID == "" {
		return "", "", status.Error(codes.PermissionDenied, "missing or invalid tenant_id in JWT claims")
	}

	// Extract device_id claim
	deviceID, ok = claims["device_id"].(string)
	if !ok || deviceID == "" {
		return "", "", status.Error(codes.PermissionDenied, "missing or invalid device_id in JWT claims")
	}

	return tenantID, deviceID, nil
}

// TenantFromJWT extracts tenant_id from JWT claims in Authorization header
// Deprecated: use ExtractClaimsFromJWT instead
// Kept for backward compatibility
func TenantFromJWT(ctx context.Context) (string, error) {
	tenantID, _, err := ExtractClaimsFromJWT(ctx)
	return tenantID, err
}

// UnaryInterceptor is a gRPC unary interceptor that extracts tenant_id and device_id from JWT
// and stores them in the request context
func UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract tenant_id and device_id from JWT
		tenantID, deviceID, err := ExtractClaimsFromJWT(ctx)
		if err != nil {
			return nil, err
		}

		// Add both tenant_id and device_id to context
		ctx = context.WithValue(ctx, TenantIDContextKey, tenantID)
		ctx = context.WithValue(ctx, DeviceIDContextKey, deviceID)

		// Call handler with enriched context
		return handler(ctx, req)
	}
}

// StreamInterceptor is a gRPC stream interceptor that extracts tenant_id and device_id from JWT
// and stores them in the request context
func StreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Extract tenant_id and device_id from JWT
		tenantID, deviceID, err := ExtractClaimsFromJWT(ss.Context())
		if err != nil {
			return err
		}

		// Add both tenant_id and device_id to context
		ctx := context.WithValue(ss.Context(), TenantIDContextKey, tenantID)
		ctx = context.WithValue(ctx, DeviceIDContextKey, deviceID)

		// Wrap the stream to use the enriched context
		wrappedStream := &wrappedServerStream{ServerStream: ss, ctx: ctx}

		// Call handler with enriched context
		return handler(srv, wrappedStream)
	}
}

// GetTenantID retrieves tenant_id from request context.
// Returns empty string if not set (e.g., during login before JWT is issued).
func GetTenantID(ctx context.Context) string {
	tenantID, _ := ctx.Value(TenantIDContextKey).(string)
	return tenantID
}

// GetDeviceID retrieves device_id from request context.
// Returns empty string if not set (e.g., during login before JWT is issued).
func GetDeviceID(ctx context.Context) string {
	deviceID, _ := ctx.Value(DeviceIDContextKey).(string)
	return deviceID
}

// SetTenantID injects tenant_id into the request context
// Used in device APIs where tenant context is determined at runtime (e.g., during login)
// Returns a new context with tenant_id set
func SetTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDContextKey, tenantID)
}

// SetDeviceID injects device_id into the request context
// Used in device APIs where device context is determined at runtime (e.g., during login)
// Returns a new context with device_id set
func SetDeviceID(ctx context.Context, deviceID string) context.Context {
	return context.WithValue(ctx, DeviceIDContextKey, deviceID)
}

// extractBearerToken extracts the token from "Bearer {token}" format
func extractBearerToken(authHeader string) string {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != BearerScheme {
		return ""
	}
	return parts[1]
}

// wrappedServerStream wraps grpc.ServerStream to provide a different context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
