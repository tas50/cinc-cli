package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandRunsVersionSubcommand(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc version failed: %v", err)
	}

	if !strings.HasPrefix(buf.String(), "cinc ") {
		t.Errorf("expected version output from `cinc version`, got:\n%s", buf.String())
	}
}

// runBareCinc executes `cinc` with no subcommand, returning combined
// stdout and stderr output.
func runBareCinc(t *testing.T, stdin string) (string, string, error) {
	t.Helper()
	root := newRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(nil)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestBareCincOffersMigrationWhenChefCredentialsPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".chef"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".chef", "credentials"), []byte("placeholder"), 0o600); err != nil {
		t.Fatal(err)
	}
	swapTTY(t, true)

	called := false
	swapMigrate(t, func(_, cincPath string) (int, error) {
		called = true
		dir := filepath.Dir(cincPath)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return 0, err
		}
		return 1, os.WriteFile(cincPath, []byte(`[default]
chef_server_url = "https://x.example.com/organizations/acme"
client_name     = "tim"
client_key      = "/k/t.pem"
`), 0o600)
	})

	stdout, stderr, err := runBareCinc(t, "y\n")
	if err != nil {
		t.Fatalf("bare cinc returned error: %v", err)
	}
	if !called {
		t.Error("expected migrateChef to be invoked on bare cinc")
	}
	if !strings.Contains(stderr, "Welcome to the cinc CLI!") {
		t.Errorf("expected welcome on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "migrate it") {
		t.Errorf("expected migration prompt on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stdout, "cinc is a unified") {
		t.Errorf("expected help text on stdout after migration, got:\n%s", stdout)
	}
}

func TestBareCincRunsConfigureWhenChefAbsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	swapTTY(t, true)
	swapMigrate(t, func(_, _ string) (int, error) {
		t.Error("migrateChef should not run when no chef file exists")
		return 0, nil
	})

	called := false
	swapConfigure(t, func(cmd *cobra.Command, cincPath string) error {
		called = true
		return fakeConfigure(t)(cmd, cincPath)
	})

	stdout, stderr, err := runBareCinc(t, "")
	if err != nil {
		t.Fatalf("bare cinc returned error: %v", err)
	}
	if !called {
		t.Error("expected runFirstRunConfigure to be invoked when no chef file exists")
	}
	if strings.Contains(stderr, "migrate it") {
		t.Errorf("did not expect a migration prompt, got stderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Welcome to the cinc CLI!") {
		t.Errorf("expected a welcome line on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "didn't find an existing Cinc or Chef config") {
		t.Errorf("expected configure-fallback intro on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stdout, "cinc is a unified") {
		t.Errorf("expected help text on stdout, got:\n%s", stdout)
	}
}

func TestBareCincSkipsMigrationWhenCincCredentialsExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.MkdirAll(filepath.Join(home, ".cinc"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".cinc", "credentials"), []byte("[default]\n"), 0o600)
	_ = os.MkdirAll(filepath.Join(home, ".chef"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".chef", "credentials"), []byte("placeholder"), 0o600)
	swapTTY(t, true)
	swapMigrate(t, func(_, _ string) (int, error) {
		t.Error("migrateChef should not run when cinc credentials already exist")
		return 0, nil
	})

	stdout, stderr, err := runBareCinc(t, "")
	if err != nil {
		t.Fatalf("bare cinc returned error: %v", err)
	}
	if strings.Contains(stderr, "migrate it") {
		t.Errorf("did not expect a migration prompt, got stderr:\n%s", stderr)
	}
	if strings.Contains(stderr, "Welcome to the cinc CLI!") {
		t.Errorf("did not expect a welcome line when cinc creds already exist, got:\n%s", stderr)
	}
	if !strings.Contains(stdout, "cinc is a unified") {
		t.Errorf("expected help text on stdout, got:\n%s", stdout)
	}
}

func TestBareCincContinuesToHelpAfterDeclinedMigration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.MkdirAll(filepath.Join(home, ".chef"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".chef", "credentials"), []byte("placeholder"), 0o600)
	swapTTY(t, true)
	swapMigrate(t, func(_, _ string) (int, error) {
		t.Error("migrateChef should not run when the user declines")
		return 0, nil
	})

	stdout, _, err := runBareCinc(t, "n\n")
	if err != nil {
		t.Fatalf("bare cinc returned error: %v", err)
	}
	if !strings.Contains(stdout, "cinc is a unified") {
		t.Errorf("expected help text on stdout after decline, got:\n%s", stdout)
	}
}
