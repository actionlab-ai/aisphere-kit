package casdoor

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNormalizedCertificateInlineCertificate(t *testing.T) {
	cert := testCertificatePEM(t)
	got, err := (Config{Certificate: "\r\n" + cert + "\r\n"}).NormalizedCertificate()
	if err != nil {
		t.Fatalf("NormalizedCertificate returned error: %v", err)
	}
	if !strings.Contains(got, "-----BEGIN CERTIFICATE-----") {
		t.Fatalf("expected certificate PEM, got %q", got[:min(len(got), 32)])
	}
}

func TestNormalizedCertificateFile(t *testing.T) {
	path := t.TempDir() + "/casdoor-public.pem"
	if err := os.WriteFile(path, []byte(testCertificatePEM(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := (Config{CertificateFile: path}).NormalizedCertificate()
	if err != nil {
		t.Fatalf("NormalizedCertificate returned error: %v", err)
	}
	if !strings.Contains(got, "-----BEGIN CERTIFICATE-----") {
		t.Fatalf("expected certificate PEM")
	}
}

func TestNormalizedCertificateRejectsPrivateKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemText := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	_, err = (Config{Certificate: pemText}).NormalizedCertificate()
	if err == nil || !strings.Contains(err.Error(), "not a private key") {
		t.Fatalf("expected private-key rejection, got %v", err)
	}
}

func TestNormalizedCertificateRejectsEmpty(t *testing.T) {
	_, err := (Config{}).NormalizedCertificate()
	if err == nil {
		t.Fatal("expected empty certificate error")
	}
}

func testCertificatePEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "cert_test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
