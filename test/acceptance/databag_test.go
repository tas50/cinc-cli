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

	human := runCinc(t, env.binary, "databag", "list", "--config", env.cfgPath)
	if human != "apps\nusers\n" {
		t.Errorf("databag list (human) = %q", human)
	}

	jsonOut := runCinc(t, env.binary, "databag", "list", "--config", env.cfgPath, "--format", "json")
	for _, name := range []string{"apps", "users"} {
		if !strings.Contains(jsonOut, name) {
			t.Errorf("databag list (json) missing %q\ngot: %s", name, jsonOut)
		}
	}
}

// TestDataBagItemListAgainstChefZero asserts the seeded "users" bag
// returns its two seed items in sorted order, and that an empty bag
// returns an empty list.
func TestDataBagItemListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "databag", "item", "list", "users", "--config", env.cfgPath)
	if human != "alice\nbob\n" {
		t.Errorf("databag item list users (human) = %q", human)
	}

	empty := runCinc(t, env.binary, "databag", "item", "list", "apps", "--config", env.cfgPath)
	if empty != "" {
		t.Errorf("databag item list apps (human) = %q, want empty", empty)
	}

	jsonOut := strings.TrimSpace(runCinc(t, env.binary, "databag", "item", "list", "apps", "--config", env.cfgPath, "--format", "json"))
	if jsonOut != "[]" {
		t.Errorf("databag item list apps (json) = %q, want \"[]\"", jsonOut)
	}
}

// TestDataBagItemListMissingBagAgainstChefZero exercises the error
// path when the bag itself does not exist on the server.
func TestDataBagItemListMissingBagAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	_, stderr, err := runCincRaw(env.binary, "databag", "item", "list", "ghosts", "--config", env.cfgPath)
	if err == nil {
		t.Fatalf("databag item list of missing bag unexpectedly succeeded")
	}
	if !strings.Contains(stderr, "404") && !strings.Contains(stderr, "not found") {
		t.Errorf("databag item list stderr does not mention 404/not found: %s", stderr)
	}
}

// TestDataBagItemEditAgainstChefZero exercises `cinc databag item
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

	out := runCinc(t, env.binary, "databag", "item", "edit", "users", "alice", "--file", itemPath, "--config", env.cfgPath)
	if out != "Updated item \"alice\" in bag \"users\"\n" {
		t.Errorf("databag item edit output = %q", out)
	}
}

func TestDataBagCreateAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := runCinc(t, env.binary, "databag", "create", "secrets", "--config", env.cfgPath)
	if out != "Created data bag \"secrets\"\n" {
		t.Errorf("databag create output = %q", out)
	}

	listed := runCinc(t, env.binary, "databag", "list", "--config", env.cfgPath)
	if !strings.Contains(listed, "secrets") {
		t.Errorf("databag list missing new bag:\n%s", listed)
	}
}

// TestDataBagCreateWithItemAgainstChefZero exercises the two-arg
// `databag create BAG ITEM` form through --file so it doesn't need
// a real terminal for the editor.
func TestDataBagCreateWithItemAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	itemPath := filepath.Join(t.TempDir(), "item.json")
	body, _ := json.Marshal(cinc.DataBagItem{"id": "db-password", "password": "hunter2"})
	if err := os.WriteFile(itemPath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	out := runCinc(t, env.binary, "databag", "create", "secrets", "db-password", "--file", itemPath, "--config", env.cfgPath)
	if !strings.Contains(out, "Created data bag \"secrets\"") {
		t.Errorf("expected bag-create line:\n%s", out)
	}
	if !strings.Contains(out, "Created item \"db-password\" in data bag \"secrets\"") {
		t.Errorf("expected item-create line:\n%s", out)
	}
}

func TestDataBagDeleteAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := runCinc(t, env.binary, "databag", "delete", "apps", "--config", env.cfgPath)
	if out != "Deleted data bag \"apps\"\n" {
		t.Errorf("databag delete output = %q", out)
	}

	after := runCinc(t, env.binary, "databag", "list", "--config", env.cfgPath)
	if after != "users\n" {
		t.Errorf("databag list after delete = %q, want apps absent", after)
	}
}
