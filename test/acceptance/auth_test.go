//go:build acceptance

package acceptance

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfigWithKey writes a credentials file for org `acme` on the
// given port using clientName and the PEM private key at keyPath, and
// returns the config file path.
func writeConfigWithKey(t *testing.T, port int, clientName, keyPath string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "credentials")
	cfg := fmt.Sprintf(`[default]
cinc_server_url = "http://127.0.0.1:%d/organizations/acme"
client_name     = %q
client_key      = %q
`, port, clientName, keyPath)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

// writeFreshKey generates a throwaway RSA private key, writes it as PEM,
// and returns its path. The key is NOT registered with the server.
func writeFreshKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "fresh.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(keyPath, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return keyPath
}

// TestAuthRejectsWrongKeyAgainstCincZero proves request signing is
// actually verified: a config signing as "pivotal" but with an
// unregistered key must be rejected (HTTP 401), not served.
func TestAuthRejectsWrongKeyAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	wrongCfg := writeConfigWithKey(t, env.port, "pivotal", writeFreshKey(t))

	_, stderr, err := runCincRaw(env.binary, "node", "list", "--config", wrongCfg)
	if err == nil {
		t.Fatalf("node list with an unregistered key unexpectedly succeeded — signatures are not being verified")
	}
	if !strings.Contains(stderr, "401") && !strings.Contains(strings.ToLower(stderr), "unauthorized") {
		t.Errorf("expected a 401/unauthorized error, got stderr:\n%s", stderr)
	}
}

// TestEnforceACLForbidsNonAdminAgainstCincZero starts cinc-zero with ACL
// enforcement on, creates a plain (non-admin) client as pivotal, then
// signs as that client and attempts an admin-only org action (creating
// an environment — the environments container is writable only by
// admins/users, not plain clients). The request authenticates (valid
// signature, and an org client *is* a recognized org actor) but must be
// refused by authorization with HTTP 403 — distinct from the 401 an
// unrecognized actor gets and the 404 a missing object yields.
func TestEnforceACLForbidsNonAdminAgainstCincZero(t *testing.T) {
	env, stop := startAcceptanceWith(t, acceptanceOptions{enforceACLs: true})
	defer stop()

	// Generate a keypair we control and register its public half as a
	// new client "robot" via the admin.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(t.TempDir(), "robot.pub")
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}), 0o644); err != nil {
		t.Fatal(err)
	}
	runCinc(t, env.binary, "client", "create", "robot", "--public-key", pubPath, "--config", env.cfgPath)

	// Write the matching private key and a config that signs as "robot".
	robotKey := filepath.Join(t.TempDir(), "robot.pem")
	if err := os.WriteFile(robotKey, pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	robotCfg := writeConfigWithKey(t, env.port, "robot", robotKey)

	// robot authenticates as an org actor but is not authorized to create
	// environments under enforce-acls.
	_, stderr, err := runCincRaw(env.binary, "environment", "create", "robotenv",
		"--description", "should be forbidden", "--config", robotCfg)
	if err == nil {
		t.Fatalf("non-admin client unexpectedly created an environment under --enforce-acls")
	}
	if !strings.Contains(stderr, "403") && !strings.Contains(strings.ToLower(stderr), "forbidden") {
		t.Errorf("expected a 403/forbidden error for a non-admin action, got stderr:\n%s", stderr)
	}
}
