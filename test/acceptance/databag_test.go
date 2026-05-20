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

func TestDataBagListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "data-bag", "list", "--config", env.cfgPath)
	if human != "apps\nusers\n" {
		t.Errorf("data-bag list (human) = %q", human)
	}

	jsonOut := runCinc(t, env.binary, "data-bag", "list", "--config", env.cfgPath, "--format", "json")
	for _, name := range []string{"apps", "users"} {
		if !strings.Contains(jsonOut, name) {
			t.Errorf("data-bag list (json) missing %q\ngot: %s", name, jsonOut)
		}
	}
}

// TestDataBagItemEditAgainstChefZero exercises `cinc data-bag item
// edit` through its `--file` path. The seed populates `users`
// with an "alice" item; the test PUTs a modified version and verifies
// the command exits cleanly.
func TestDataBagItemEditAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	itemPath := filepath.Join(t.TempDir(), "alice.json")
	body, err := json.Marshal(cinc.DataBagItem{"id": "alice", "role": "editor"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(itemPath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	out := runCinc(t, env.binary, "data-bag", "item", "edit", "users", "alice", "--file", itemPath, "--config", env.cfgPath)
	if out != "Updated item \"alice\" in bag \"users\"\n" {
		t.Errorf("data-bag item edit output = %q", out)
	}
}

func TestDataBagDeleteAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := runCinc(t, env.binary, "data-bag", "delete", "apps", "--config", env.cfgPath)
	if out != "Deleted data bag \"apps\"\n" {
		t.Errorf("data-bag delete output = %q", out)
	}

	after := runCinc(t, env.binary, "data-bag", "list", "--config", env.cfgPath)
	if after != "users\n" {
		t.Errorf("data-bag list after delete = %q, want apps absent", after)
	}
}
