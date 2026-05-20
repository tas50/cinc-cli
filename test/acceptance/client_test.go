//go:build acceptance

package acceptance

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

// TestClientListAgainstChefZero asserts the seeded clients are
// returned. chef-zero may add additional clients of its own (e.g. an
// auto-created validator); the test only requires the seeded names to
// be present.
func TestClientListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "client", "list", "--config", env.cfgPath)
	for _, name := range []string{"worker-01", "worker-02"} {
		if !strings.Contains(human, name) {
			t.Errorf("client list (human) missing %q\ngot: %s", name, human)
		}
	}

	jsonOut := runCinc(t, env.binary, "client", "list", "--config", env.cfgPath, "--format", "json")
	for _, name := range []string{"worker-01", "worker-02"} {
		if !strings.Contains(jsonOut, name) {
			t.Errorf("client list (json) missing %q\ngot: %s", name, jsonOut)
		}
	}
}

// TestClientCreateAgainstChefZero exercises `cinc client create` using
// the BYO public-key path. The default (server-generated key) path is
// avoided here because chef-zero returns the generated key at the top
// level of the response (`{"private_key": ...}`) rather than nested
// under `chef_key`, which is what cinc-api unmarshals — so the CLI's
// key-bearing branches are not reachable against chef-zero. That
// response-shape mismatch is exercised by the unit tests against an
// in-process httptest server that returns the modern shape.
func TestClientCreateAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	pubPath := filepath.Join(t.TempDir(), "fresh.pub")
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		t.Fatal(err)
	}

	out := runCinc(t, env.binary, "client", "create", "fresh", "--public-key", pubPath, "--config", env.cfgPath)
	if out != "Created client \"fresh\"\n" {
		t.Errorf("client create output = %q", out)
	}

	listed := runCinc(t, env.binary, "client", "list", "--config", env.cfgPath)
	if !strings.Contains(listed, "fresh") {
		t.Errorf("client list after create missing %q\ngot: %s", "fresh", listed)
	}
}

// TestClientEditAgainstChefZero exercises `cinc client edit` through
// its `--file` path. The built-in TUI editor branch is unreachable
// from `go test` (no real terminal attached), so the acceptance test
// covers the scripted path; the TUI editor branch is exercised by
// hand and by the unit tests that stub editJSON.
func TestClientEditAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	keyPath := filepath.Join(t.TempDir(), "client.json")
	body, err := json.Marshal(cinc.APIClient{Name: "worker-01", Validator: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	out := runCinc(t, env.binary, "client", "edit", "worker-01", "--file", keyPath, "--config", env.cfgPath)
	if out != "Updated client \"worker-01\"\n" {
		t.Errorf("client edit output = %q", out)
	}

	// The seeded client name should still be in `client list` — edit
	// must not have created a new client or dropped the existing one.
	listed := runCinc(t, env.binary, "client", "list", "--config", env.cfgPath)
	if !strings.Contains(listed, "worker-01") {
		t.Errorf("client list after edit missing worker-01:\n%s", listed)
	}
}

// TestClientDeleteAgainstChefZero deletes a seeded client and confirms
// it is gone from the follow-up list.
func TestClientDeleteAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := runCinc(t, env.binary, "client", "delete", "worker-02", "--config", env.cfgPath)
	if out != "Deleted client \"worker-02\"\n" {
		t.Errorf("client delete output = %q", out)
	}

	listed := runCinc(t, env.binary, "client", "list", "--config", env.cfgPath)
	if strings.Contains(listed, "worker-02") {
		t.Errorf("client list still contains %q after delete:\n%s", "worker-02", listed)
	}
	if !strings.Contains(listed, "worker-01") {
		t.Errorf("client list lost unrelated %q after delete:\n%s", "worker-01", listed)
	}
}
