package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleConfig = `
default_profile = "prod"

[profiles.prod]
server_url = "https://chef.example.com"
org = "acme"
client_name = "tim"
key_path = "/keys/tim.pem"

[profiles.staging]
server_url = "https://staging.example.com"
org = "acme-staging"
client_name = "tim"
key_path = "/keys/staging.pem"
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadParsesProfiles(t *testing.T) {
	cfg, err := Load(writeConfig(t, sampleConfig))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DefaultProfile != "prod" {
		t.Errorf("DefaultProfile = %q, want %q", cfg.DefaultProfile, "prod")
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(cfg.Profiles))
	}
	prod := cfg.Profiles["prod"]
	if prod.ServerURL != "https://chef.example.com" || prod.Org != "acme" ||
		prod.ClientName != "tim" || prod.KeyPath != "/keys/tim.pem" {
		t.Errorf("prod profile parsed incorrectly: %+v", prod)
	}
}

func TestConfigProfileResolvesDefaultWhenNameEmpty(t *testing.T) {
	cfg, _ := Load(writeConfig(t, sampleConfig))

	p, err := cfg.Profile("")
	if err != nil {
		t.Fatalf("Profile(\"\"): %v", err)
	}
	if p.Org != "acme" {
		t.Errorf("default profile Org = %q, want %q", p.Org, "acme")
	}
}

func TestConfigProfileResolvesByName(t *testing.T) {
	cfg, _ := Load(writeConfig(t, sampleConfig))

	p, err := cfg.Profile("staging")
	if err != nil {
		t.Fatalf("Profile(staging): %v", err)
	}
	if p.Org != "acme-staging" {
		t.Errorf("staging profile Org = %q, want %q", p.Org, "acme-staging")
	}
}

func TestConfigProfileUnknownNameErrors(t *testing.T) {
	cfg, _ := Load(writeConfig(t, sampleConfig))

	if _, err := cfg.Profile("does-not-exist"); err == nil {
		t.Error("expected an error resolving an unknown profile")
	}
}

func TestConfigProfileNoNameAndNoDefaultErrors(t *testing.T) {
	cfg, _ := Load(writeConfig(t, "[profiles.only]\norg = \"o\"\n"))

	if _, err := cfg.Profile(""); err == nil {
		t.Error("expected an error when no profile is named and no default is set")
	}
}

func TestProfileValidate(t *testing.T) {
	complete := Profile{ServerURL: "u", Org: "o", ClientName: "c", KeyPath: "k"}
	if err := complete.Validate(); err != nil {
		t.Errorf("complete profile should validate, got: %v", err)
	}

	incomplete := []Profile{
		{Org: "o", ClientName: "c", KeyPath: "k"},
		{ServerURL: "u", ClientName: "c", KeyPath: "k"},
		{ServerURL: "u", Org: "o", KeyPath: "k"},
		{ServerURL: "u", Org: "o", ClientName: "c"},
	}
	for _, p := range incomplete {
		if err := p.Validate(); err == nil {
			t.Errorf("incomplete profile %+v should fail validation", p)
		}
	}
}

func TestDefaultPathPointsAtCincConfigFile(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(".cinc", "config.toml")
	if !strings.HasSuffix(p, want) {
		t.Errorf("DefaultPath = %q, want it to end with %q", p, want)
	}
}
