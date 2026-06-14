package client

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tas50/cinc-cli/cli/config"
)

// writeKeyFile generates an RSA key, writes it as PEM to a temp file, and
// returns the path.
func writeKeyFile(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewRejectsIncompleteProfile(t *testing.T) {
	if _, err := New(config.Profile{Org: "acme"}); err == nil {
		t.Error("expected an error building a client from an incomplete profile")
	}
}

func TestNewRejectsMissingKeyFile(t *testing.T) {
	p := config.Profile{
		ServerURL:  "https://chef.example.com",
		Org:        "acme",
		ClientName: "tim",
		KeyPath:    filepath.Join(t.TempDir(), "absent.pem"),
	}
	if _, err := New(p); err == nil {
		t.Error("expected an error when the key file does not exist")
	}
}

func TestNewBuildsClientFromValidProfile(t *testing.T) {
	p := config.Profile{
		ServerURL:  "https://chef.example.com",
		Org:        "acme",
		ClientName: "tim",
		KeyPath:    writeKeyFile(t),
	}

	c, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c == nil {
		t.Fatal("New returned a nil client")
	}
}

// withCapturedTLSWarn swaps tlsWarnWriter for a buffer for the test's duration.
func withCapturedTLSWarn(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := tlsWarnWriter
	tlsWarnWriter = &buf
	t.Cleanup(func() { tlsWarnWriter = orig })
	return &buf
}

func TestNewWarnsWhenTLSVerificationDisabled(t *testing.T) {
	buf := withCapturedTLSWarn(t)
	p := config.Profile{
		ServerURL:     "https://chef.example.com",
		Org:           "acme",
		ClientName:    "tim",
		KeyPath:       writeKeyFile(t),
		SSLVerifyMode: ":verify_none",
	}
	if _, err := New(p); err != nil {
		t.Fatalf("New: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "verification is disabled") || !strings.Contains(got, "chef.example.com") {
		t.Errorf("expected a TLS-disabled warning naming the server, got %q", got)
	}
}

func TestNewDoesNotWarnWhenTLSVerified(t *testing.T) {
	buf := withCapturedTLSWarn(t)
	p := config.Profile{
		ServerURL:  "https://chef.example.com",
		Org:        "acme",
		ClientName: "tim",
		KeyPath:    writeKeyFile(t),
	}
	if _, err := New(p); err != nil {
		t.Fatalf("New: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("did not expect a warning with verification on, got %q", buf.String())
	}
}
