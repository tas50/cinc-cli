package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExploreCommandRequiresTTY drives `cinc explore` through cobra with
// a valid credentials file but non-terminal output. The command should
// load config, build Options, and reach the TUI's TTY guard — proving
// the wiring end-to-end without an interactive terminal.
func TestExploreCommandRequiresTTY(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "credentials")
	cfg := fmt.Sprintf(`[default]
cinc_server_url = "https://example.test/organizations/acme"
client_name     = "tim"
client_key      = %q
`, writeTestKey(t))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"explore", "--config", cfgPath})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("err = %v, want an interactive-terminal message", err)
	}
}
