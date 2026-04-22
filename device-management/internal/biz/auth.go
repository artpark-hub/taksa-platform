package biz

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/sha3"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// AuthUsecase handles authentication and token operations
type AuthUsecase struct {
	store     storage.Store
	jwtSecret string
}

// NewAuthUsecase creates a new auth use case.
// JWT secret is sourced from server config (TAKSA_DM_JWT_SECRET / server.jwt_secret).
func NewAuthUsecase(store storage.Store, serverConf *conf.Server) *AuthUsecase {
	return &AuthUsecase{
		store:     store,
		jwtSecret: serverConf.JwtSecret,
	}
}

// JWTSecret returns the signing secret so callers (e.g., middleware) can
// verify tokens signed by this service without duplicating the source.
func (uc *AuthUsecase) JWTSecret() string {
	return uc.jwtSecret
}

// ValidateAuthToken validates a token hash by iterating stored tokens and hashing them (SYSTEM-WIDE SEARCH)
// CRITICAL: Used by Login endpoint (device API, which doesn't have tenant context)
// Flow: Client sends SHA3(SHA3(rawToken)) → iterate all tokens, hash each → compare
// Returns: (deviceID, tenantID, rawToken, error)
// Both deviceID and tenantID are resolved from the auth_tokens table in one lookup.
func (uc *AuthUsecase) ValidateAuthToken(ctx context.Context, clientHash string) (string, string, string, error) {
	if clientHash == "" {
		return "", "", "", fmt.Errorf("token hash is empty")
	}

	// Get all valid tokens across all tenants (system-wide search)
	// Safe because tokens are cryptographically unique; only ONE matching token can exist
	validTokens, err := uc.store.AuthTokens().GetAllValidAuthTokensSystemWide(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to retrieve tokens: %w", err)
	}

	// Iterate through all tokens and hash them to find a match
	// This is how we validate: client sends hash(hash(rawToken)), we hash the raw token and compare
	for rawToken, info := range validTokens {
		computedHash := hashToken(rawToken)
		if computedHash == clientHash {
			return info.DeviceID, info.TenantID, rawToken, nil
		}
	}

	return "", "", "", fmt.Errorf("invalid token: no matching token found")
}

// GenerateJWT generates a JWT token for a device
// Includes tenant_id from context so all device APIs can enforce tenant isolation
func (uc *AuthUsecase) GenerateJWT(ctx context.Context, device *v1.Device) (string, error) {
	if device == nil || device.Id == "" {
		return "", fmt.Errorf("device is nil or has no ID")
	}

	// Multi-tenancy: extract tenant_id from JWT context (set by middleware or during device registration)
	// This is required for all device API calls to be tenant-isolated
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return "", fmt.Errorf("tenant_id not found in context - cannot generate JWT without tenant isolation")
	}

	// Create JWT claims with tenant_id for device API isolation
	claims := jwt.MapClaims{
		"device_id":   device.Id,
		"device_name": device.Name,
		"tenant_id":   tenantID,  // Multi-tenancy: device APIs use this to enforce tenant isolation
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
	}

	// Sign token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(uc.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return tokenString, nil
}

// VerifyJWT verifies a JWT token and extracts claims
func (uc *AuthUsecase) VerifyJWT(tokenString string) (map[string]interface{}, error) {
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(uc.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid JWT: %w", err)
	}

	return claims, nil
}

// ExtractDeviceIDFromJWT extracts the device_id claim from a JWT token string
// Returns the device ID if valid, or empty string + error if invalid
func (uc *AuthUsecase) ExtractDeviceIDFromJWT(tokenString string) (string, error) {
	if tokenString == "" {
		return "", fmt.Errorf("JWT token is empty")
	}

	claims, err := uc.VerifyJWT(tokenString)
	if err != nil {
		return "", fmt.Errorf("failed to verify JWT: %w", err)
	}

	deviceID, ok := claims["device_id"].(string)
	if !ok || deviceID == "" {
		return "", fmt.Errorf("device_id not found or invalid in JWT claims")
	}

	return deviceID, nil
}

// CreateAuthToken creates a new auth token for device registration
// Returns: raw token (shown to user and stored in DB)
// The raw token will be hashed during login validation to match client-sent double hash
// Tokens are valid for 50 years from registration with no automatic renewal
// tenantID is required for multi-tenancy isolation
func (uc *AuthUsecase) CreateAuthToken(ctx context.Context, tenantID, deviceID string, expiryDays int) (token string, err error) {
	if tenantID == "" || deviceID == "" {
		return "", fmt.Errorf("tenant ID or device ID is empty")
	}

	// Generate random token (32 bytes = 64 hex chars)
	token = generateRandomToken()

	// Create auth token entity
	authToken := &models.AuthToken{
		Token:     token,
		DeviceID:  deviceID,
		ExpiresAt: time.Now().AddDate(50, 0, 0), // 50 year expiry from registration
		CreatedAt: time.Now(),
	}

	// Save raw token to storage (not hashed) with tenant isolation
	// During login, we'll retrieve all tokens and hash them to compare with client-sent hash
	err = uc.store.AuthTokens().Save(ctx, tenantID, authToken, token)
	if err != nil {
		return "", fmt.Errorf("failed to save auth token: %w", err)
	}

	return token, nil
}

// RenewAuthToken extends an auth token's expiry by another 50 years (called on successful login)
// This ensures active devices never expire as long as they login at least once every 50 years
// tenantID is required for multi-tenancy isolation
func (uc *AuthUsecase) RenewAuthToken(ctx context.Context, tenantID, token string) error {
	if tenantID == "" || token == "" {
		return fmt.Errorf("tenant ID or token is empty")
	}

	// Update expiry to 50 years from now with tenant isolation
	newExpiryTime := time.Now().AddDate(50, 0, 0)
	err := uc.store.AuthTokens().UpdateExpiry(ctx, tenantID, token, newExpiryTime)
	if err != nil {
		return fmt.Errorf("failed to update auth token expiry: %w", err)
	}

	return nil
}

// Helper functions

// generateRandomToken generates a random 32-byte token (64 hex chars)
func generateRandomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback in case of error (should not happen)
		panic(fmt.Sprintf("failed to generate random token: %v", err))
	}
	return fmt.Sprintf("%x", b)
}

// hashToken returns SHA3-256(SHA3-256(token)) - double hash
// This is what the client sends in the Authorization header
// Must match UMH Core's LoginHash implementation
// NOTE: The second hash operates on the HEX-ENCODED string of the first hash, not the raw bytes
func hashToken(token string) string {
	// First SHA3-256 hash of the token
	hash1 := sha3.New256()
	hash1.Write([]byte(token))
	hash1Hex := fmt.Sprintf("%x", hash1.Sum(nil))
	
	// Second SHA3-256 hash of the hex string
	hash2 := sha3.New256()
	hash2.Write([]byte(hash1Hex))
	return fmt.Sprintf("%x", hash2.Sum(nil))
}

// hashTokenRaw returns SHA3-256(token) - single hash
// Used for intermediate validation
// Must match UMH Core's TokenHash implementation
func hashTokenRaw(token string) string {
	hash := sha3.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}
