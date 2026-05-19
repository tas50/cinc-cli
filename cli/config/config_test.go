package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleConfig = `
[default]
client_name     = "tim"
client_key      = "/keys/tim.pem"
cinc_server_url = "https://cinc.example.com/organizations/acme"

[staging]
client_name     = "tim"
client_key      = "/keys/staging.pem"
cinc_server_url = "https://staging.example.com/organizations/acme-staging"
ssl_verify_mode = ":verify_none"
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "credentials")
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

	if len(cfg.Profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(cfg.Profiles))
	}
	prod := cfg.Profiles["default"]
	if prod.ServerURL != "https://cinc.example.com" || prod.Org != "acme" ||
		prod.ClientName != "tim" || prod.KeyPath != "/keys/tim.pem" {
		t.Errorf("default profile parsed incorrectly: %+v", prod)
	}
	staging := cfg.Profiles["staging"]
	if staging.SSLVerifyMode != ":verify_none" {
		t.Errorf("staging ssl_verify_mode = %q, want %q", staging.SSLVerifyMode, ":verify_none")
	}
}

func TestConfigProfileResolvesDefaultWhenNameEmpty(t *testing.T) {
	t.Setenv("CHEF_PROFILE", "")
	cfg, _ := Load(writeConfig(t, sampleConfig))

	p, err := cfg.Profile("")
	if err != nil {
		t.Fatalf("Profile(\"\"): %v", err)
	}
	if p.Org != "acme" {
		t.Errorf("default profile Org = %q, want %q", p.Org, "acme")
	}
}

func TestConfigProfileHonorsChefProfileEnv(t *testing.T) {
	t.Setenv("CINC_PROFILE", "")
	t.Setenv("CHEF_PROFILE", "staging")
	cfg, _ := Load(writeConfig(t, sampleConfig))

	p, err := cfg.Profile("")
	if err != nil {
		t.Fatalf("Profile(\"\"): %v", err)
	}
	if p.Org != "acme-staging" {
		t.Errorf("env-selected profile Org = %q, want %q", p.Org, "acme-staging")
	}
}

func TestConfigProfileHonorsCincProfileEnv(t *testing.T) {
	t.Setenv("CINC_PROFILE", "staging")
	t.Setenv("CHEF_PROFILE", "")
	cfg, _ := Load(writeConfig(t, sampleConfig))

	p, err := cfg.Profile("")
	if err != nil {
		t.Fatalf("Profile(\"\"): %v", err)
	}
	if p.Org != "acme-staging" {
		t.Errorf("CINC_PROFILE-selected profile Org = %q, want %q", p.Org, "acme-staging")
	}
}

func TestConfigProfileCincEnvWinsOverChefEnv(t *testing.T) {
	t.Setenv("CINC_PROFILE", "staging")
	t.Setenv("CHEF_PROFILE", "default")
	cfg, _ := Load(writeConfig(t, sampleConfig))

	p, err := cfg.Profile("")
	if err != nil {
		t.Fatalf("Profile(\"\"): %v", err)
	}
	if p.Org != "acme-staging" {
		t.Errorf("with both env vars set, expected CINC_PROFILE to win; got Org = %q", p.Org)
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

func TestLoadAcceptsChefServerURLKey(t *testing.T) {
	cfg, err := Load(writeConfig(t, `
[default]
client_name     = "tim"
client_key      = "/keys/tim.pem"
chef_server_url = "https://chef.example.com/organizations/acme"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := cfg.Profiles["default"]
	if p.ServerURL != "https://chef.example.com" || p.Org != "acme" {
		t.Errorf("chef_server_url (compat form) parsed incorrectly: %+v", p)
	}
}

func TestLoadCincServerURLWinsOverChefServerURL(t *testing.T) {
	cfg, err := Load(writeConfig(t, `
[default]
client_name     = "tim"
client_key      = "/keys/tim.pem"
chef_server_url = "https://chef.example.com/organizations/chef-org"
cinc_server_url = "https://cinc.example.com/organizations/cinc-org"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := cfg.Profiles["default"]
	if p.ServerURL != "https://cinc.example.com" || p.Org != "cinc-org" {
		t.Errorf("with both keys set, expected cinc_server_url to win; got %+v", p)
	}
}

func TestLoadRejectsServerURLWithoutOrganizationSegment(t *testing.T) {
	bad := `
[default]
client_name     = "tim"
client_key      = "/keys/tim.pem"
cinc_server_url = "https://cinc.example.com"
`
	if _, err := Load(writeConfig(t, bad)); err == nil {
		t.Error("expected an error for a server URL missing /organizations/<org>")
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

func TestDefaultPathPointsAtCincCredentialsFile(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(".cinc", "credentials")
	if !strings.HasSuffix(p, want) {
		t.Errorf("DefaultPath = %q, want it to end with %q", p, want)
	}
}
