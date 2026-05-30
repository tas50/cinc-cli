//go:build acceptance

package acceptance

import (
	"encoding/json"
	"strings"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

// TestUserListAgainstChefZero verifies that `cinc user list` against a
// real chef-zero server returns the seeded global users (alongside the
// default "pivotal" superuser) in both human and JSON output formats.
func TestUserListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "user", "list", "--config", env.cfgPath)
	for _, name := range []string{"anna", "ben"} {
		if !strings.Contains(human, name) {
			t.Errorf("user list (human) missing %q\ngot: %s", name, human)
		}
	}

	jsonOut := runCinc(t, env.binary, "user", "list", "--config", env.cfgPath, "--format", "json")
	for _, name := range []string{"anna", "ben"} {
		if !strings.Contains(jsonOut, name) {
			t.Errorf("user list (json) missing %q\ngot: %s", name, jsonOut)
		}
	}
}

// TestUserShowAgainstChefZero fetches a seeded user and asserts on both
// the default (pretty JSON) and `--format json` outputs.
func TestUserShowAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "user", "show", "anna", "--config", env.cfgPath)
	if !strings.Contains(human, "anna") {
		t.Errorf("user show (human) missing username:\n%s", human)
	}

	jsonOut := runCinc(t, env.binary, "user", "show", "anna", "--config", env.cfgPath, "--format", "json")
	var got cinc.User
	if err := json.Unmarshal([]byte(jsonOut), &got); err != nil {
		t.Fatalf("user show (json) not valid JSON: %v\n%s", err, jsonOut)
	}
	if got.UserName != "anna" {
		t.Errorf("user show returned username=%q, want anna", got.UserName)
	}
}
