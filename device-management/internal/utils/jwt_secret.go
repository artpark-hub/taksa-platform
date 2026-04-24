package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultJWTSecretFile is the default path for persisting the JWT secret under /data
	DefaultJWTSecretFile = "/data/jwt.secret"
	// JWTSecretLength is the length of the random secret in bytes (32 bytes = 256 bits)
	JWTSecretLength = 32
)

// GetOrGenerateJWTSecret returns a JWT signing secret.
// Priority:
// 1. If envSecret is not empty, use it (TAKSA_DM_JWT_SECRET from env/config)
// 2. If /data/jwt.secret exists, read and return it
// 3. Otherwise, generate a new random secret, persist it to /data/jwt.secret, and return it
//
// This ensures session continuity across service restarts while avoiding
// the need to manually configure TAKSA_DM_JWT_SECRET for each deployment.
func GetOrGenerateJWTSecret(envSecret string) (string, error) {
	return GetOrGenerateJWTSecretWithPath(envSecret, DefaultJWTSecretFile)
}

// GetOrGenerateJWTSecretWithPath is like GetOrGenerateJWTSecret but allows
// specifying a custom path for the persisted secret (useful for testing).
func GetOrGenerateJWTSecretWithPath(envSecret, filePath string) (string, error) {
	// 1. Explicit configuration takes highest priority
	if envSecret != "" {
		return envSecret, nil
	}

	// 2. Check for persisted secret
	secret, err := readJWTSecretFromFile(filePath)
	if err == nil {
		return secret, nil
	}
	// If file doesn't exist, we'll generate a new one
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to read JWT secret file: %w", err)
	}

	// 3. Generate and persist a new secret
	secret, err = generateAndPersistJWTSecret(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT secret: %w", err)
	}

	return secret, nil
}

// readJWTSecretFromFile reads the JWT secret from a file and trims whitespace/newlines
// to prevent issues when secrets are created with echo or have trailing newlines
func readJWTSecretFromFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", fmt.Errorf("%w: JWT secret file is empty or contains only whitespace", os.ErrNotExist)
	}

	return secret, nil
}

// generateAndPersistJWTSecret generates a random secret and writes it to a file
func generateAndPersistJWTSecret(filePath string) (string, error) {
	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Generate random bytes
	bytes := make([]byte, JWTSecretLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hex string
	secret := hex.EncodeToString(bytes)

	// Write to file with restrictive permissions (owner read/write only)
	if err := os.WriteFile(filePath, []byte(secret), 0600); err != nil {
		return "", fmt.Errorf("failed to write JWT secret to %s: %w", filePath, err)
	}

	return secret, nil
}
