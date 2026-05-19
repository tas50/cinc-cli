package client

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
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
