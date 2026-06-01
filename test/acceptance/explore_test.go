//go:build acceptance

package acceptance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExploreHelpListsCommand confirms the real binary registers the
// explore command and renders its help.
func TestExploreHelpListsCommand(t *testing.T) {
	binary := buildCinc(t)

	root := runCinc(t, binary, "--help")
	if !strings.Contains(root, "explore") {
		t.Errorf("root help does not list explore:\n%s", root)
	}

	help := runCinc(t, binary, "explore", "--help")
	for _, want := range []string{"terminal UI", "object type"} {
		if !strings.Contains(help, want) {
			t.Errorf("explore help missing %q\ngot: %s", want, help)
		}
	}
}

// TestExploreRequiresTTY confirms the real binary refuses to launch the
// TUI when stdout is not an interactive terminal. The acceptance
// harness captures stdout into a buffer (never a TTY), so this is the
// one explore path it can exercise — the interactive flow itself is
// covered by the model-level unit tests in cli/explore, as the TUI
// cannot be driven through this harness.
func TestExploreRequiresTTY(t *testing.T) {
	binary := buildCinc(t)

	// A minimal but parseable credentials file. The TTY guard fires
	// before any profile is resolved or any key is read, so the key
	// path need not be valid.
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(keyPath, []byte("unused"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "credentials")
	cfg := fmt.Sprintf(`[default]
cinc_server_url = "http://127.0.0.1:1/organizations/acme"
client_name     = "tester"
client_key      = %q
`, keyPath)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runCincRaw(binary, "explore", "--config", cfgPath)
	if err == nil {
		t.Fatal("expected a non-zero exit when stdout is not a TTY")
	}
	if !strings.Contains(stderr, "interactive terminal") {
		t.Errorf("stderr = %q, want an interactive-terminal message", stderr)
	}
}
