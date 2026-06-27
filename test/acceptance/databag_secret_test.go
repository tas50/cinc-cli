//go:build acceptance

package acceptance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

// TestDataBagSecretRoundTripAgainstCincZero creates an encrypted item,
// reads it back decrypted with the same secret, and confirms that a plain
// `databag item show` still sees the encrypted-at-rest wrapper.
func TestDataBagSecretRoundTripAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	// The bag must already exist, just like `databag item create`.
	runCinc(t, env.binary, "databag", "create", "secrets", "--config", env.cfgPath)

	secretPath := filepath.Join(t.TempDir(), "encrypted_data_bag_secret")
	if err := os.WriteFile(secretPath, []byte("open-sesame"), 0o600); err != nil {
		t.Fatal(err)
	}

	itemPath := filepath.Join(t.TempDir(), "item.json")
	body, _ := json.Marshal(cinc.DataBagItem{"id": "db-password", "password": "hunter2"})
	if err := os.WriteFile(itemPath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	out := runCinc(t, env.binary, "databag", "secret", "create", "secrets", "db-password",
		"--file", itemPath, "--secret-file", secretPath, "--config", env.cfgPath)
	if out != "Created encrypted item \"db-password\" in data bag \"secrets\"\n" {
		t.Errorf("databag secret create output = %q", out)
	}

	// Reading it back with the same secret decrypts the plaintext.
	shown := runCinc(t, env.binary, "databag", "secret", "show", "secrets", "db-password",
		"--secret-file", secretPath, "--config", env.cfgPath, "--format", "json")
	var plain cinc.DataBagItem
	if err := json.Unmarshal([]byte(shown), &plain); err != nil {
		t.Fatalf("databag secret show output not valid JSON: %v\n%s", err, shown)
	}
	if plain["id"] != "db-password" || plain["password"] != "hunter2" {
		t.Errorf("decrypted item = %+v, want id=db-password password=hunter2", plain)
	}

	// A plain `databag item show` sees the still-encrypted wrapper at rest.
	atRest := runCinc(t, env.binary, "databag", "item", "show", "secrets", "db-password",
		"--config", env.cfgPath, "--format", "json")
	var encrypted cinc.DataBagItem
	if err := json.Unmarshal([]byte(atRest), &encrypted); err != nil {
		t.Fatalf("databag item show output not valid JSON: %v\n%s", err, atRest)
	}
	if encrypted["id"] != "db-password" {
		t.Errorf("encrypted item id = %v, want cleartext db-password", encrypted["id"])
	}
	if pw, ok := encrypted["password"].(string); ok {
		t.Errorf("password stored in cleartext at rest: %q", pw)
	}
	if !encrypted.IsEncrypted() {
		t.Errorf("item is not encrypted at rest: %+v", encrypted)
	}
	// The plaintext must not appear anywhere in the at-rest payload.
	if strings.Contains(atRest, "hunter2") {
		t.Errorf("plaintext leaked into encrypted-at-rest payload:\n%s", atRest)
	}
}
