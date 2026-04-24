package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetOrGenerateJWTSecret(t *testing.T) {
	tests := []struct {
		name           string
		envSecret      string
		setupFile      bool
		expectedErr    bool
		expectedSecret string
		description    string
	}{
		{
			name:           "env_secret_takes_priority",
			envSecret:      "explicit-secret-from-env",
			setupFile:      false,
			expectedErr:    false,
			expectedSecret: "explicit-secret-from-env",
			description:    "Should use env secret if provided",
		},
		{
			name:           "generate_and_persist",
			envSecret:      "",
			setupFile:      false,
			expectedErr:    false,
			expectedSecret: "", // We'll check it's non-empty and hex
			description:    "Should generate and persist a new secret",
		},
		{
			name:           "read_persisted_secret",
			envSecret:      "",
			setupFile:      true,
			expectedErr:    false,
			expectedSecret: "pre-existing-secret-value",
			description:    "Should read persisted secret from file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tmpDir := t.TempDir()
			testFilePath := filepath.Join(tmpDir, "jwt.secret")

			// Setup file if needed
			if tt.setupFile {
				if err := os.WriteFile(testFilePath, []byte(tt.expectedSecret), 0600); err != nil {
					t.Fatalf("failed to setup test file: %v", err)
				}
			}

			// Call the function with custom path
			secret, err := GetOrGenerateJWTSecretWithPath(tt.envSecret, testFilePath)

			// Check error
			if tt.expectedErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check secret value
			if tt.expectedSecret != "" {
				if secret != tt.expectedSecret {
					t.Errorf("expected %q, got %q", tt.expectedSecret, secret)
				}
			} else if tt.name == "generate_and_persist" {
				// For generated secrets, check it's non-empty and looks like hex
				if secret == "" {
					t.Errorf("expected non-empty secret, got empty string")
				}
				if len(secret) != 64 { // 32 bytes * 2 hex chars
					t.Errorf("expected hex secret length 64, got %d", len(secret))
				}
				// Verify it's valid hex
				for _, c := range secret {
					if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
						t.Errorf("expected valid hex characters, got invalid char: %c", c)
					}
				}
			}

			// For generation test, verify file was created
			if tt.name == "generate_and_persist" {
				if _, err := os.Stat(testFilePath); err != nil {
					t.Errorf("expected file to be created at %s, but got error: %v", testFilePath, err)
				}
			}
		})
	}
}

func TestGetOrGenerateJWTSecretPersistence(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	testFilePath := filepath.Join(tmpDir, "jwt.secret")

	// First call should generate and persist
	secret1, err := GetOrGenerateJWTSecretWithPath("", testFilePath)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call should return the same secret
	secret2, err := GetOrGenerateJWTSecretWithPath("", testFilePath)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Secrets should match
	if secret1 != secret2 {
		t.Errorf("expected same secret on subsequent calls, got %q then %q", secret1, secret2)
	}

	// Verify file permissions are restrictive
	info, err := os.Stat(testFilePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	mode := info.Mode()
	if mode&0077 != 0 { // Check that group and others have no permissions
		t.Errorf("expected file permissions 0600, got %o", mode.Perm())
	}
}

func TestGetOrGenerateJWTSecretTrimsWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	testFilePath := filepath.Join(tmpDir, "jwt.secret")

	// Write secret with trailing newline (common with echo command)
	secretValue := "my-secret-value"
	if err := os.WriteFile(testFilePath, []byte(secretValue+"\n"), 0600); err != nil {
		t.Fatalf("failed to setup test file: %v", err)
	}

	// Should trim and return without newline
	secret, err := GetOrGenerateJWTSecretWithPath("", testFilePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if secret != secretValue {
		t.Errorf("expected %q, got %q (newline not trimmed)", secretValue, secret)
	}
}

func TestGetOrGenerateJWTSecretRejectsWhitespaceOnly(t *testing.T) {
	tmpDir := t.TempDir()
	testFilePath := filepath.Join(tmpDir, "jwt.secret")

	// Write only whitespace/newlines
	if err := os.WriteFile(testFilePath, []byte("   \n  \t  \n"), 0600); err != nil {
		t.Fatalf("failed to setup test file: %v", err)
	}

	// Should fail because file is whitespace-only
	_, err := GetOrGenerateJWTSecretWithPath("", testFilePath)
	if err == nil {
		t.Errorf("expected error for whitespace-only file, got nil")
	}
}
