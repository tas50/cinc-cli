package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// fakeCmd builds a cobra command with the same flags resolveProfile
// reads, plus a stdin/stderr wired to the provided buffers so tests
// can drive the interactive prompts.
func fakeCmd(configFlag, profileFlag, stdin string, stderr *bytes.Buffer) *cobra.Command {
	c := &cobra.Command{Use: "fake"}
	c.Flags().String("config", configFlag, "")
	c.Flags().String("profile", profileFlag, "")
	c.Flags().String("format", "human", "")
	c.SetIn(strings.NewReader(stdin))
	c.SetOut(new(bytes.Buffer))
	c.SetErr(stderr)
	return c
}

// swapTTY sets stdinIsTTY for the duration of one test.
func swapTTY(t *testing.T, isTTY bool) {
	t.Helper()
	prev := stdinIsTTY
	stdinIsTTY = func() bool { return isTTY }
	t.Cleanup(func() { stdinIsTTY = prev })
}

// swapMigrate sets migrateChef for the duration of one test.
func swapMigrate(t *testing.T, fn func(chefPath, cincPath string) (int, error)) {
	t.Helper()
	prev := migrateChef
	migrateChef = fn
	t.Cleanup(func() { migrateChef = prev })
}

// seedDefaultCreds writes a minimal valid credentials file at
// $HOME/.cinc/credentials and returns the home tempdir.
func seedDefaultCreds(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".cinc")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := `[default]
chef_server_url = "https://x.example.com/organizations/acme"
client_name     = "tim"
client_key      = "/k/t.pem"
`
	if err := os.WriteFile(filepath.Join(dir, "credentials"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestResolveProfileLoadsDefaultPathWhenFlagEmpty(t *testing.T) {
	seedDefaultCreds(t)
	c := fakeCmd("", "", "", new(bytes.Buffer))

	p, err := resolveProfile(c)
	if err != nil {
		t.Fatalf("resolveProfile: %v", err)
	}
	if p.Org != "acme" || p.ClientName != "tim" {
		t.Errorf("unexpected profile: %+v", p)
	}
}

func TestResolveProfileSkipsMigrationWhenExplicitConfigMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	explicit := filepath.Join(t.TempDir(), "no-such.toml")

	called := false
	swapMigrate(t, func(_, _ string) (int, error) {
		called = true
		return 0, nil
	})

	c := fakeCmd(explicit, "", "", new(bytes.Buffer))
	if _, err := resolveProfile(c); err == nil {
		t.Error("expected an error loading the explicit missing config")
	}
	if called {
		t.Error("migrateChef must not run when --config is set explicitly")
	}
}

func TestResolveProfileWelcomesUserOnFirstRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	swapTTY(t, true)
	swapMigrate(t, func(_, _ string) (int, error) { return 0, nil })

	stderr := new(bytes.Buffer)
	c := fakeCmd("", "", "", stderr)
	_, _ = resolveProfile(c)
	if !strings.Contains(stderr.String(), "Welcome to cinc!") {
		t.Errorf("expected a welcome line on stderr, got:\n%s", stderr.String())
	}
}

func TestResolveProfileRunsMigrationWhenDefaultMissingAndChefExists(t *testing.T) {
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
		// Write a usable cinc file so the retry succeeds.
		dir := filepath.Dir(cincPath)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return 0, err
		}
		body := `[default]
chef_server_url = "https://x.example.com/organizations/acme"
client_name     = "tim"
client_key      = "/k/t.pem"
`
		return 1, os.WriteFile(cincPath, []byte(body), 0o600)
	})

	stderr := new(bytes.Buffer)
	c := fakeCmd("", "", "y\n", stderr)
	p, err := resolveProfile(c)
	if err != nil {
		t.Fatalf("resolveProfile: %v", err)
	}
	if !called {
		t.Error("expected migrateChef to be invoked")
	}
	if p.Org != "acme" {
		t.Errorf("post-migration profile = %+v", p)
	}
	if !strings.Contains(stderr.String(), "migrate it") {
		t.Errorf("expected migration prompt on stderr, got:\n%s", stderr.String())
	}
}

func TestResolveProfileAcceptsBlankAnswerAsYes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.MkdirAll(filepath.Join(home, ".chef"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".chef", "credentials"), []byte("x"), 0o600)
	swapTTY(t, true)

	called := false
	swapMigrate(t, func(_, cincPath string) (int, error) {
		called = true
		_ = os.MkdirAll(filepath.Dir(cincPath), 0o700)
		return 1, os.WriteFile(cincPath, []byte(`[default]
chef_server_url = "https://x.example.com/organizations/acme"
client_name     = "tim"
client_key      = "/k/t.pem"
`), 0o600)
	})

	c := fakeCmd("", "", "\n", new(bytes.Buffer))
	if _, err := resolveProfile(c); err != nil {
		t.Fatalf("resolveProfile: %v", err)
	}
	if !called {
		t.Error("blank answer should default to yes; expected migrateChef call")
	}
}

func TestResolveProfileDeclinedMigrationPointsAtConfigure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.MkdirAll(filepath.Join(home, ".chef"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".chef", "credentials"), []byte("x"), 0o600)
	swapTTY(t, true)

	swapMigrate(t, func(_, _ string) (int, error) {
		t.Error("migrateChef should not be called when the user declines")
		return 0, nil
	})

	stderr := new(bytes.Buffer)
	c := fakeCmd("", "", "n\n", stderr)
	_, err := resolveProfile(c)
	if err == nil || !strings.Contains(err.Error(), "cinc configure") {
		t.Errorf("expected an error mentioning `cinc configure`, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "No problem") {
		t.Errorf("expected a friendly acknowledgement after decline, got:\n%s", stderr.String())
	}
}

func TestResolveProfilePointsAtConfigureWhenNoChefFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	swapTTY(t, true)
	swapMigrate(t, func(_, _ string) (int, error) {
		t.Error("migrateChef should not be called when ~/.chef/credentials is absent")
		return 0, nil
	})

	c := fakeCmd("", "", "", new(bytes.Buffer))
	_, err := resolveProfile(c)
	if err == nil || !strings.Contains(err.Error(), "cinc configure") {
		t.Errorf("expected an error mentioning `cinc configure`, got: %v", err)
	}
}

func TestResolveProfilePointsAtConfigureWhenStdinNotTTY(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.MkdirAll(filepath.Join(home, ".chef"), 0o700)
	_ = os.WriteFile(filepath.Join(home, ".chef", "credentials"), []byte("x"), 0o600)
	swapTTY(t, false)
	swapMigrate(t, func(_, _ string) (int, error) {
		t.Error("migrateChef should not be called when stdin is not a TTY")
		return 0, nil
	})

	c := fakeCmd("", "", "y\n", new(bytes.Buffer))
	_, err := resolveProfile(c)
	if err == nil || !strings.Contains(err.Error(), "cinc configure") {
		t.Errorf("expected an error mentioning `cinc configure`, got: %v", err)
	}
}
