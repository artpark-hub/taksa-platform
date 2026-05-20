package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ctxKey is an unexported type for context keys to avoid collisions across packages.
// All access goes through the exported Get/Set helpers.
type ctxKey string

const (
	AuthorizationHeader        = "authorization"
	TenantIDContextKey  ctxKey = "tenant_id"
	DeviceIDContextKey  ctxKey = "device_id"
	// AuthorizationContextKey holds the bearer credential for Login (raw auth token hash).
	// Same string value as service.AuthorizationKey for HTTP/gRPC Login handlers.
	AuthorizationContextKey ctxKey = "authorization"
	// deviceJWTVerifiedCtxKey is set when the request carried a device JWT whose signature
	// was verified by this service (HTTP cookie, verified Authorization HS256 from Login, or gRPC equivalent).
	// Edge Instance APIs require this before handlers run (see grpcAuthContext and HTTP tenant middleware).
	deviceJWTVerifiedCtxKey ctxKey = "device_jwt_signature_verified"
	BearerScheme                   = "Bearer"

	grpcInstanceServicePrefix = "/api.umh_core.v2.InstanceService/"
)

const grpcJWTLeeway = 2 * time.Minute

// MarkDeviceJWTSignatureVerified returns a child context that marks the device JWT as
// cryptographically verified (issued by Login, same secret as Verify paths).
func MarkDeviceJWTSignatureVerified(ctx context.Context) context.Context {
	return context.WithValue(ctx, deviceJWTVerifiedCtxKey, true)
}

// IsDeviceJWTSignatureVerified reports whether the device identity came from a
// signature-verified device JWT (not merely parsed/unverified bearer claims).
// Instance device traffic enforces this at the HTTP and gRPC boundaries; callers may
// still use this for defense in depth or tests.
func IsDeviceJWTSignatureVerified(ctx context.Context) bool {
	v, _ := ctx.Value(deviceJWTVerifiedCtxKey).(bool)
	return v
}

func isGRPCInstanceLogin(fullMethod string) bool {
	return strings.HasSuffix(fullMethod, "InstanceService/Login")
}

// SetAuthorizationToken stores the Login bearer credential (auth token hash) in context.
func SetAuthorizationToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, AuthorizationContextKey, token)
}

// GetAuthorizationToken returns the Login bearer credential from context.
func GetAuthorizationToken(ctx context.Context) string {
	v, _ := ctx.Value(AuthorizationContextKey).(string)
	return v
}

func authorizationFromGRPCMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	authHeaders := md.Get(AuthorizationHeader)
	if len(authHeaders) == 0 {
		return ctx
	}
	if token := extractBearerToken(authHeaders[0]); token != "" {
		return SetAuthorizationToken(ctx, token)
	}
	return ctx
}

// tryVerifyDeviceJWTHS256 validates a JWT issued by device-management Login (HS256, server secret).
func tryVerifyDeviceJWTHS256(tokenString, jwtSecret string) (tenantID, deviceID string, ok bool) {
	if jwtSecret == "" || tokenString == "" {
		return "", "", false
	}
	claims := jwt.MapClaims{}
	parser := jwt.NewParser(
		jwt.WithLeeway(grpcJWTLeeway),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	token, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return "", "", false
	}
	tid, _ := claims["tenant_id"].(string)
	did, _ := claims["device_id"].(string)
	if tid == "" || did == "" {
		return "", "", false
	}
	return tid, did, true
}

func grpcAuthContext(ctx context.Context, fullMethod, jwtSecret string) (context.Context, error) {
	// Login uses the raw auth token (not a JWT); extract bearer hash for the handler.
	if isGRPCInstanceLogin(fullMethod) {
		return authorizationFromGRPCMetadata(ctx), nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeaders := md.Get(AuthorizationHeader)
	if len(authHeaders) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	token := extractBearerToken(authHeaders[0])
	if token == "" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	var tenantID, deviceID string
	verified := false

	if tid, did, ok := tryVerifyDeviceJWTHS256(token, jwtSecret); ok {
		tenantID, deviceID, verified = tid, did, true
	} else {
		claims := jwt.MapClaims{}
		_, _, err := new(jwt.Parser).ParseUnverified(token, claims)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("failed to parse JWT: %v", err))
		}
		var tok bool
		tenantID, tok = claims["tenant_id"].(string)
		if !tok || tenantID == "" {
			return nil, status.Error(codes.PermissionDenied, "missing or invalid tenant_id in JWT claims")
		}
		deviceID, _ = claims["device_id"].(string)
	}

	if strings.HasPrefix(fullMethod, grpcInstanceServicePrefix) {
		if deviceID == "" {
			return nil, status.Error(codes.PermissionDenied, "missing or invalid device_id in JWT claims")
		}
	}

	ctx = context.WithValue(ctx, TenantIDContextKey, tenantID)
	if deviceID != "" {
		ctx = context.WithValue(ctx, DeviceIDContextKey, deviceID)
	}
	if verified {
		ctx = MarkDeviceJWTSignatureVerified(ctx)
	}
	// Edge Instance RPCs must not rely on forgeable unverified JWT claims.
	if strings.HasPrefix(fullMethod, grpcInstanceServicePrefix) && !isGRPCInstanceLogin(fullMethod) && !verified {
		return nil, status.Error(codes.Unauthenticated,
			"instance service requires a Login-issued device JWT (HS256) verified by this service")
	}
	return ctx, nil
}

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
	// NOTE: device_id is optional for console/user JWTs (e.g., DeviceMgmtService),
	// but required for instance/device JWTs (InstanceService). Enforcement is done
	// in the gRPC interceptors based on the called method.
	if !ok {
		deviceID = ""
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

// UnaryInterceptor returns a gRPC unary interceptor that extracts tenant_id and device_id from JWT
// and stores them in the request context. When jwtSecret is set, HS256 Login-issued JWTs are
// signature-verified and IsDeviceJWTSignatureVerified(ctx) is true for those requests.
func UnaryInterceptor(jwtSecret string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ctx, err := grpcAuthContext(ctx, info.FullMethod, jwtSecret)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor mirrors UnaryInterceptor for streaming RPCs.
func StreamInterceptor(jwtSecret string) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx, err := grpcAuthContext(ss.Context(), info.FullMethod, jwtSecret)
		if err != nil {
			return err
		}
		wrappedStream := &wrappedServerStream{ServerStream: ss, ctx: ctx}
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
