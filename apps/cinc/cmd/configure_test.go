package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tas50/cinc-cli/cli/config"
)

func TestConfigureCommandWritesTOMLCredentialsProfile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "credentials")
	keyPath := filepath.Join(dir, "damacus.pem")
	if err := os.WriteFile(keyPath, []byte("-----BEGIN RSA PRIVATE KEY-----\n-----END RSA PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"configure",
		"--server-url", "https://api.chef.io/organizations/damacus",
		"--supermarket-site", "https://supermarket.chef.io",
		"--client-name", "damacus",
		"--client-key", keyPath,
		"--profile", "supermarket",
		"--config", cfgPath,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc configure: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load credentials: %v", err)
	}
	p := cfg.Profiles["supermarket"]
	if p.ServerURL != "https://api.chef.io" || p.Org != "damacus" || p.SupermarketSite != "https://supermarket.chef.io" ||
		p.ClientName != "damacus" || p.KeyPath != keyPath {
		t.Fatalf("profile = %+v", p)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "client.rb") || !strings.Contains(string(data), "chef_server_url") ||
		!strings.Contains(string(data), "supermarket_site") {
		t.Fatalf("credentials = %s, want TOML credentials with chef_server_url, supermarket_site, and no client.rb", data)
	}
	if got := buf.String(); !strings.Contains(got, `Wrote credentials profile "supermarket"`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestConfigureCommandAcceptsSupermarketURLAsServerURL(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "credentials")
	keyPath := filepath.Join(dir, "damacus.pem")
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{
		"configure",
		"--server-url", "https://supermarket.chef.io",
		"--client-name", "damacus",
		"--client-key", keyPath,
		"--config", cfgPath,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc configure: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load credentials: %v", err)
	}
	p := cfg.Profiles["supermarket"]
	if p.ServerURL != "" || p.Org != "" || p.SupermarketSite != "https://supermarket.chef.io" {
		t.Fatalf("profile = %+v, want supermarket-only profile", p)
	}
}

func TestConfigureCommandSecondRunMutatesSameProfile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "credentials")
	firstKeyPath := filepath.Join(dir, "first.pem")
	secondKeyPath := filepath.Join(dir, "second.pem")
	for _, path := range []string{firstKeyPath, secondKeyPath} {
		if err := os.WriteFile(path, []byte("key"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	first := newRootCmd()
	first.SetOut(&bytes.Buffer{})
	first.SetArgs([]string{
		"configure",
		"--server-url", "https://api.chef.io/organizations/old-org",
		"--client-name", "old-client",
		"--client-key", firstKeyPath,
		"--profile", "supermarket",
		"--config", cfgPath,
	})
	if err := first.Execute(); err != nil {
		t.Fatalf("first cinc configure: %v", err)
	}

	second := newRootCmd()
	second.SetOut(&bytes.Buffer{})
	second.SetArgs([]string{
		"configure",
		"--server-url", "https://supermarket.chef.io",
		"--client-name", "damacus",
		"--client-key", secondKeyPath,
		"--profile", "supermarket",
		"--config", cfgPath,
	})
	if err := second.Execute(); err != nil {
		t.Fatalf("second cinc configure: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load credentials: %v", err)
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("profiles = %+v, want only the mutated supermarket profile", cfg.Profiles)
	}
	p := cfg.Profiles["supermarket"]
	if p.ServerURL != "" || p.Org != "" || p.SupermarketSite != "https://supermarket.chef.io" ||
		p.ClientName != "damacus" || p.KeyPath != secondKeyPath {
		t.Fatalf("profile = %+v, want second run to replace first run values", p)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "old-org") || strings.Contains(string(data), firstKeyPath) ||
		strings.Contains(string(data), "chef_server_url") {
		t.Fatalf("credentials = %s, want stale first-run values removed", data)
	}
}

func TestConfigureCommandOnboardsWithDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USER", "damacus")

	keyPath := filepath.Join(home, ".ssh", "damacus.pem")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(home, ".cinc", "credentials")

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetIn(strings.NewReader("\n\n\n\n\n\n\n"))
	root.SetArgs([]string{"configure", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc configure: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load credentials: %v", err)
	}
	p := cfg.Profiles["supermarket"]
	if p.SupermarketSite != "https://supermarket.chef.io" || p.ClientName != "damacus" || p.KeyPath != keyPath {
		t.Fatalf("profile = %+v", p)
	}
	stdout := out.String()
	for _, want := range []string{
		"Credentials file location [" + cfgPath + "]",
		"Profile name [default]",
		"Supermarket site [https://supermarket.chef.io]",
		"Client key path [" + keyPath + "]",
		"Chef server host (optional, e.g. chef.example.com) []",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want prompt %q", stdout, want)
		}
	}
}

func TestConfigureCommandPreservesExistingProfiles(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "credentials")
	keyPath := filepath.Join(dir, "new.pem")
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteProfile(cfgPath, "default", config.Profile{
		ServerURL:  "https://old.example.test",
		Org:        "old",
		ClientName: "old",
		KeyPath:    "/keys/old.pem",
	}); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{
		"configure",
		"--chef-server-url", "https://api.chef.io/organizations/damacus",
		"--client-name", "damacus",
		"--client-key", keyPath,
		"--profile", "damacus",
		"--config", cfgPath,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc configure: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load credentials: %v", err)
	}
	if _, ok := cfg.Profiles["default"]; !ok {
		t.Fatal("default profile was not preserved")
	}
	if cfg.Profiles["damacus"].ClientName != "damacus" {
		t.Fatalf("damacus profile = %+v", cfg.Profiles["damacus"])
	}
}

func TestConfigureCommandRejectsMissingClientKey(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{
		"configure",
		"--server-url", "https://api.chef.io/organizations/damacus",
		"--client-name", "damacus",
		"--client-key", filepath.Join(t.TempDir(), "missing.pem"),
		"--config", filepath.Join(t.TempDir(), "credentials"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected missing key to fail")
	}
	if !strings.Contains(err.Error(), "read client key") {
		t.Fatalf("error = %q", err)
	}
}
