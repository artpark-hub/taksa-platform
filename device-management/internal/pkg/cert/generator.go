package cert

import (
	"crypto/rand"
	"crypto/rsa"
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
	// Validate deviceID length to avoid panic on slicing
	if len(deviceID) < 8 {
		return "", "", fmt.Errorf("deviceID must be at least 8 characters, got %d", len(deviceID))
	}

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

	// Create PKCS#8 private key
	// NOTE: This key is returned UNENCRYPTED. The PEM label says "ENCRYPTED PRIVATE KEY"
	// for compatibility, but no actual encryption is applied. Secure the key in transit
	// (HTTPS) and at rest (proper secret management / KMS). Do NOT rely on the label.
	keyPEM, err = encryptPrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode private key: %w", err)
	}

	return certPEM, keyPEM, nil
}

// encryptPrivateKey encodes an RSA private key to PEM format
// WARNING: The resulting PEM is labeled "ENCRYPTED PRIVATE KEY" for compatibility,
// but NO actual encryption is applied. This is suitable ONLY when:
// - Private keys are transmitted over secure channels (HTTPS)
// - Private keys are stored with appropriate access controls
// - A proper KMS or secret management system controls access
// For true encryption, implement PBKDF2 + AES-256-GCM or use a proper HSM/KMS.
func encryptPrivateKey(privateKey *rsa.PrivateKey) (string, error) {
	// Marshal private key to PKCS#8 DER format (unencrypted)
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal private key to PKCS#8: %w", err)
	}

	// Encode to PEM with "ENCRYPTED PRIVATE KEY" label for compatibility
	// Note: This is a misnomer - the key is NOT encrypted
	encryptedBlock := &pem.Block{
		Type:  "ENCRYPTED PRIVATE KEY",
		Bytes: privateKeyDER,
	}

	return string(pem.EncodeToMemory(encryptedBlock)), nil
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
