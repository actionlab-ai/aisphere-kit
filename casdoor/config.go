package casdoor

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Endpoint     string `json:"endpoint" yaml:"endpoint"`
	ClientID     string `json:"client_id" yaml:"client_id"`
	ClientSecret string `json:"client_secret" yaml:"client_secret"`
	// Certificate is the PEM encoded public certificate/public key used to
	// verify Casdoor access tokens. It must match the JWT certificate selected
	// by the Casdoor Application. Do not put Casdoor's private key here.
	Certificate string `json:"certificate" yaml:"certificate"`
	// CertificateFile points to a PEM file containing the public certificate or
	// public key used to verify Casdoor access tokens. Certificate takes
	// precedence when both fields are set.
	CertificateFile string `json:"certificate_file" yaml:"certificate_file"`
	Organization    string `json:"organization" yaml:"organization"`
	Application     string `json:"application" yaml:"application"`
	// Authz selector fields map to Casdoor /api/enforce query selectors.
	// Casdoor expects exactly one selector for each enforce call.
	// Configure PermissionID first in most cases; ModelID/ResourceID/EnforcerID/Owner are fallback selectors.
	PermissionID string `json:"permission_id" yaml:"permission_id"`
	ModelID      string `json:"model_id" yaml:"model_id"`
	ResourceID   string `json:"resource_id" yaml:"resource_id"`
	EnforcerID   string `json:"enforcer_id" yaml:"enforcer_id"`
	Owner        string `json:"owner" yaml:"owner"`
	// PolicyEnforcer and PolicyAdapterID are used by Casdoor/Casbin policy-management APIs.
	// Configure PolicyEnforcer to the Casdoor enforcer name that owns the Casbin adapter.
	PolicyEnforcer  string `json:"policy_enforcer" yaml:"policy_enforcer"`
	PolicyAdapterID string `json:"policy_adapter_id" yaml:"policy_adapter_id"`
	HTTPTimeout     string `json:"http_timeout" yaml:"http_timeout"`
	DefaultScope    string `json:"default_scope" yaml:"default_scope"`
	AllowAnonymous  bool   `json:"allow_anonymous" yaml:"allow_anonymous"`
	AuditAsync      bool   `json:"audit_async" yaml:"audit_async"`
	AuditQueue      int    `json:"audit_queue" yaml:"audit_queue"`
	AuditTimeout    string `json:"audit_timeout" yaml:"audit_timeout"`
	RetryAttempts   int    `json:"retry_attempts" yaml:"retry_attempts"`
	RetryBackoff    string `json:"retry_backoff" yaml:"retry_backoff"`
}

// NormalizedCertificate returns a trimmed PEM certificate/public key suitable
// for casdoor-go-sdk.NewClient. Inline certificate content has priority over a
// file path so config overlays and secrets managers can override mounted files.
func (c Config) NormalizedCertificate() (string, error) {
	cert := strings.TrimSpace(c.Certificate)
	if cert == "" && strings.TrimSpace(c.CertificateFile) != "" {
		b, err := os.ReadFile(strings.TrimSpace(c.CertificateFile))
		if err != nil {
			return "", fmt.Errorf("read casdoor.certificate_file %q: %w", c.CertificateFile, err)
		}
		cert = strings.TrimSpace(string(b))
	}
	cert = normalizePEMText(cert)
	if cert == "" {
		return "", fmt.Errorf("casdoor.certificate or casdoor.certificate_file is required for authn JWT verification; copy the public certificate/public key from the Casdoor Cert page")
	}
	if err := ValidateJWTPublicCertificate(cert); err != nil {
		return "", err
	}
	return cert, nil
}

// ValidateJWTPublicCertificate checks that pemText contains a public certificate
// or public key usable for verifying RS256 Casdoor JWTs. It intentionally rejects
// private keys: services consuming Casdoor tokens should only need the public key.
func ValidateJWTPublicCertificate(pemText string) error {
	cert := normalizePEMText(pemText)
	if cert == "" {
		return fmt.Errorf("casdoor.certificate is empty")
	}
	block, rest := pem.Decode([]byte(cert))
	if block == nil {
		return fmt.Errorf("casdoor.certificate must be PEM encoded public certificate or public key")
	}
	if strings.TrimSpace(string(rest)) != "" {
		// Additional PEM blocks are not usually needed and are often accidental
		// copy/paste noise. Keep this strict so bad cert config fails at startup.
		return fmt.Errorf("casdoor.certificate must contain exactly one PEM block")
	}

	switch block.Type {
	case "CERTIFICATE":
		parsed, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("parse casdoor.certificate x509 certificate: %w", err)
		}
		if _, ok := parsed.PublicKey.(*rsa.PublicKey); !ok {
			return fmt.Errorf("casdoor.certificate certificate public key must be RSA for RS256 JWT verification")
		}
		return nil
	case "PUBLIC KEY":
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("parse casdoor.certificate public key: %w", err)
		}
		if _, ok := pub.(*rsa.PublicKey); !ok {
			return fmt.Errorf("casdoor.certificate public key must be RSA for RS256 JWT verification")
		}
		return nil
	case "RSA PUBLIC KEY":
		if _, err := x509.ParsePKCS1PublicKey(block.Bytes); err != nil {
			return fmt.Errorf("parse casdoor.certificate rsa public key: %w", err)
		}
		return nil
	case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY":
		return fmt.Errorf("casdoor.certificate must be a public certificate/public key, not a private key")
	default:
		return fmt.Errorf("casdoor.certificate unsupported PEM block type %q; expected CERTIFICATE, PUBLIC KEY, or RSA PUBLIC KEY", block.Type)
	}
}

func normalizePEMText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// Some secret stores flatten newlines as literal \n. Accept that shape too.
	if strings.Contains(s, `\n`) && !strings.Contains(s, "\n") {
		s = strings.ReplaceAll(s, `\n`, "\n")
	}
	return strings.TrimSpace(s)
}
