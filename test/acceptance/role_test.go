//go:build acceptance

package acceptance

import (
	"encoding/json"
	"strings"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

func TestRoleListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "role", "list", "--config", env.cfgPath)
	if human != "base\ndb\nweb\n" {
		t.Errorf("role list (human) = %q, want sorted role names", human)
	}

	jsonOut := runCinc(t, env.binary, "role", "list", "--config", env.cfgPath, "--format", "json")
	for _, name := range []string{"base", "db", "web"} {
		if !strings.Contains(jsonOut, name) {
			t.Errorf("role list (json) missing %q\ngot: %s", name, jsonOut)
		}
	}
}

// TestRoleShowAgainstChefZero fetches a seeded role and asserts on both
// the default (pretty JSON) and `--format json` outputs.
func TestRoleShowAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "role", "show", "web", "--config", env.cfgPath)
	if !strings.Contains(human, "\"name\": \"web\"") {
		t.Errorf("role show (human) missing name field:\n%s", human)
	}

	jsonOut := runCinc(t, env.binary, "role", "show", "web", "--config", env.cfgPath, "--format", "json")
	var got cinc.Role
	if err := json.Unmarshal([]byte(jsonOut), &got); err != nil {
		t.Fatalf("role show (json) not valid JSON: %v\n%s", err, jsonOut)
	}
	if got.Name != "web" {
		t.Errorf("role show returned name=%q, want web", got.Name)
	}
}

func TestRoleDeleteAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := runCinc(t, env.binary, "role", "delete", "web", "--config", env.cfgPath)
	if out != "Deleted role \"web\"\n" {
		t.Errorf("role delete output = %q", out)
	}

	after := runCinc(t, env.binary, "role", "list", "--config", env.cfgPath)
	if after != "base\ndb\n" {
		t.Errorf("role list after delete = %q, want web absent", after)
	}
}
