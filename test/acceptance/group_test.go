//go:build acceptance

package acceptance

import (
	"encoding/json"
	"strings"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

// TestGroupListAgainstChefZero verifies that `cinc group list` against
// a real chef-zero server returns the default per-org groups in both
// human and JSON output formats.
func TestGroupListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "group", "list", "--config", env.cfgPath)
	for _, name := range []string{"admins", "clients", "users"} {
		if !strings.Contains(human, name) {
			t.Errorf("group list (human) missing %q\ngot: %s", name, human)
		}
	}

	jsonOut := runCinc(t, env.binary, "group", "list", "--config", env.cfgPath, "--format", "json")
	if !strings.Contains(jsonOut, "admins") {
		t.Errorf("group list (json) missing admins\ngot: %s", jsonOut)
	}
}

// TestGroupShowAgainstChefZero fetches the default "admins" group and
// asserts on both the default (pretty JSON) and `--format json`
// outputs.
func TestGroupShowAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "group", "show", "admins", "--config", env.cfgPath)
	if !strings.Contains(human, "admins") {
		t.Errorf("group show (human) missing groupname:\n%s", human)
	}

	jsonOut := runCinc(t, env.binary, "group", "show", "admins", "--config", env.cfgPath, "--format", "json")
	var got cinc.Group
	if err := json.Unmarshal([]byte(jsonOut), &got); err != nil {
		t.Fatalf("group show (json) not valid JSON: %v\n%s", err, jsonOut)
	}
	if got.GroupName != "admins" {
		t.Errorf("group show returned groupname=%q, want admins", got.GroupName)
	}
}
