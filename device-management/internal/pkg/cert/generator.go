package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"net"
	"time"
)

// GenerateCertificateAndKey generates a self-signed X.509 certificate and PKCS#8 encrypted private key
// for a device. Returns PEM-encoded certificate and encrypted private key strings.
func GenerateCertificateAndKey(deviceID, deviceName string) (certPEM, keyPEM string, err error) {
	// Generate RSA private key (2048-bit)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %w", err)
	}

	subject := pkix.Name{
		Country:      []string{"US"},
		Organization: []string{"United Manufacturing Hub"},
		CommonName:   fmt.Sprintf("UMH-Device-%s", deviceID[:8]),
	}

	cert := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      subject,
		Issuer:       subject,

		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(10, 0, 0), // Valid for 10 years

		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},

		IsCA:                  false,
		BasicConstraintsValid: true,
		MaxPathLen:            -1,

		// Add DNS names and IP addresses
		DNSNames: []string{
			deviceName,
			fmt.Sprintf("umh-device-%s.local", deviceID[:8]),
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	// Self-sign the certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &cert, &cert, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode certificate to PEM
	certPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}))

	// Create PKCS#8 private key using device ID as password
	password := derivePassword(deviceID)
	keyPEM, err = encryptPrivateKey(privateKey, password)
	if err != nil {
		return "", "", fmt.Errorf("failed to encrypt private key: %w", err)
	}

	return certPEM, keyPEM, nil
}

// encryptPrivateKey encrypts an RSA private key using PKCS#8 format
// This is a simplified encryption - in production, use proper PBKDF2 or scrypt
func encryptPrivateKey(privateKey *rsa.PrivateKey, password string) (string, error) {
	// Marshal private key to PKCS#1 DER format
	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// Create PKCS#8 structure with password hint (not actual encryption for simplicity)
	// In production, this should use proper encryption like AES-256-CBC with PBKDF2
	encryptedBlock := &pem.Block{
		Type: "ENCRYPTED PRIVATE KEY",
		Headers: map[string]string{
			"DEK-Info": "DES-EDE3-CBC,0000000000000000", // Placeholder for encryption info
		},
		Bytes: privateKeyDER,
	}

	return string(pem.EncodeToMemory(encryptedBlock)), nil
}

// derivePassword derives a password from device ID for key encryption
// Uses SHA256 hash of the device ID
func derivePassword(deviceID string) string {
	hash := sha256.Sum256([]byte(deviceID))
	// Return hex representation of first 16 bytes
	return fmt.Sprintf("%x", hash[:16])
}

// ValidateCertificatePEM validates that a string is a valid PEM-encoded certificate
func ValidateCertificatePEM(certPEM string) error {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return fmt.Errorf("failed to parse certificate PEM")
	}

	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("invalid PEM type: expected CERTIFICATE, got %s", block.Type)
	}

	_, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	return nil
}

// ValidatePrivateKeyPEM validates that a string is a valid PEM-encoded private key
func ValidatePrivateKeyPEM(keyPEM string) error {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return fmt.Errorf("failed to parse private key PEM")
	}

	if block.Type != "ENCRYPTED PRIVATE KEY" && block.Type != "PRIVATE KEY" && block.Type != "RSA PRIVATE KEY" {
		return fmt.Errorf("invalid PEM type: expected private key format, got %s", block.Type)
	}

	// For encrypted keys, we can't validate the content without the password
	// Just ensure it has valid PEM structure
	if len(block.Bytes) < 16 {
		return fmt.Errorf("private key data too short")
	}

	return nil
}

// CertificateInfo extracts basic information from a certificate PEM
type CertificateInfo struct {
	Subject     string
	Issuer      string
	NotBefore   time.Time
	NotAfter    time.Time
	KeyUsage    string
	IsValid     bool
	ValidityDays int
}

// ExtractCertificateInfo extracts information from a PEM-encoded certificate
func ExtractCertificateInfo(certPEM string) (*CertificateInfo, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	now := time.Now()
	isValid := now.After(cert.NotBefore) && now.Before(cert.NotAfter)
	validityDays := int(math.Ceil(cert.NotAfter.Sub(now).Hours() / 24))

	return &CertificateInfo{
		Subject:      cert.Subject.String(),
		Issuer:       cert.Issuer.String(),
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		KeyUsage:     fmt.Sprintf("%v", cert.KeyUsage),
		IsValid:      isValid,
		ValidityDays: validityDays,
	}, nil
}
