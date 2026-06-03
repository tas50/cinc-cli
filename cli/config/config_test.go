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

[supermarket]
client_name      = "tim"
client_key       = "/keys/supermarket.pem"
supermarket_site = "https://supermarket.chef.io"

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

	if len(cfg.Profiles) != 3 {
		t.Fatalf("got %d profiles, want 3", len(cfg.Profiles))
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
	supermarket := cfg.Profiles["supermarket"]
	if supermarket.SupermarketSite != "https://supermarket.chef.io" || supermarket.KeyPath != "/keys/supermarket.pem" {
		t.Errorf("supermarket profile parsed incorrectly: %+v", supermarket)
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

func TestNewProfileParsesConfigureValues(t *testing.T) {
	p, err := NewProfile("https://cinc.example.com/organizations/acme", "worker", "/keys/worker.pem", ":verify_peer", "https://supermarket.example.test")
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	if p.ServerURL != "https://cinc.example.com" || p.Org != "acme" || p.ClientName != "worker" ||
		p.KeyPath != "/keys/worker.pem" || p.SSLVerifyMode != ":verify_peer" || p.SupermarketSite != "https://supermarket.example.test" {
		t.Fatalf("profile = %+v", p)
	}
}

func TestNewProfileTreatsServerURLWithoutOrganizationAsSupermarketSite(t *testing.T) {
	p, err := NewProfile("https://supermarket.chef.io", "worker", "/keys/worker.pem", "", "")
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	if p.ServerURL != "" || p.Org != "" || p.SupermarketSite != "https://supermarket.chef.io" {
		t.Fatalf("profile = %+v, want supermarket-only profile", p)
	}
}

func TestWriteProfileCreatesCredentialsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".cinc", "credentials")

	err := WriteProfile(path, "worker", Profile{
		ServerURL:       "https://cinc.example.com",
		Org:             "acme",
		SupermarketSite: "https://supermarket.example.test",
		ClientName:      "worker",
		KeyPath:         "/keys/worker.pem",
	})
	if err != nil {
		t.Fatalf("WriteProfile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credentials mode = %o, want 0600", perm)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load written credentials: %v", err)
	}
	p := cfg.Profiles["worker"]
	if p.ServerURL != "https://cinc.example.com" || p.Org != "acme" || p.SupermarketSite != "https://supermarket.example.test" || p.ClientName != "worker" || p.KeyPath != "/keys/worker.pem" {
		t.Fatalf("written profile = %+v", p)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "chef_server_url") || !strings.Contains(string(data), "supermarket_site") {
		t.Fatalf("credentials = %s, want chef_server_url and supermarket_site", data)
	}
}

func TestWriteProfileAllowsSupermarketOnlyProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".cinc", "credentials")

	err := WriteProfile(path, "supermarket", Profile{
		SupermarketSite: "https://supermarket.chef.io",
		ClientName:      "worker",
		KeyPath:         "/keys/worker.pem",
	})
	if err != nil {
		t.Fatalf("WriteProfile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load written credentials: %v", err)
	}
	p := cfg.Profiles["supermarket"]
	if p.ServerURL != "" || p.Org != "" || p.SupermarketSite != "https://supermarket.chef.io" {
		t.Fatalf("written profile = %+v", p)
	}
}

func TestWriteProfilePreservesExistingProfiles(t *testing.T) {
	path := writeConfig(t, sampleConfig)

	err := WriteProfile(path, "worker", Profile{
		ServerURL:  "https://cinc.example.com",
		Org:        "acme",
		ClientName: "worker",
		KeyPath:    "/keys/worker.pem",
	})
	if err != nil {
		t.Fatalf("WriteProfile: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load written credentials: %v", err)
	}
	if _, ok := cfg.Profiles["default"]; !ok {
		t.Fatal("default profile was not preserved")
	}
	if _, ok := cfg.Profiles["staging"]; !ok {
		t.Fatal("staging profile was not preserved")
	}
	if cfg.Profiles["worker"].ClientName != "worker" {
		t.Fatalf("worker profile = %+v", cfg.Profiles["worker"])
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

func TestProfileValidateIdentity(t *testing.T) {
	complete := Profile{ClientName: "c", KeyPath: "k"}
	if err := complete.ValidateIdentity(); err != nil {
		t.Errorf("complete identity should validate, got: %v", err)
	}

	incomplete := []Profile{
		{KeyPath: "k"},
		{ClientName: "c"},
	}
	for _, p := range incomplete {
		if err := p.ValidateIdentity(); err == nil {
			t.Errorf("incomplete identity %+v should fail validation", p)
		}
	}
}

func TestConfigValidateReportsProfileIssues(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"broken": {
			SSLVerifyMode: "nope",
		},
		"supermarket": {
			ClientName:      "tim",
			KeyPath:         "/keys/tim.pem",
			SupermarketSite: "https://supermarket.chef.io",
		},
	}}

	issues := cfg.Validate()
	for _, want := range []string{"client_name", "client_key", "endpoint", "ssl_verify_mode"} {
		if !hasValidationIssue(issues, "broken", want) {
			t.Fatalf("issues = %#v, want field %q", issues, want)
		}
	}
	if hasValidationIssue(issues, "supermarket", "endpoint") {
		t.Fatalf("issues = %#v, supermarket-only profile should validate", issues)
	}
}

func TestConfigValidateRejectsEmptyConfig(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}

	issues := cfg.Validate()
	if len(issues) != 1 || issues[0].Field != "profiles" {
		t.Fatalf("issues = %#v", issues)
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

func hasValidationIssue(issues []ValidationIssue, profile, field string) bool {
	for _, issue := range issues {
		if issue.Profile == profile && issue.Field == field {
			return true
		}
	}
	return false
}
