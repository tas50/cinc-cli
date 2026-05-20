//go:build acceptance

package acceptance

import (
	"strings"
	"testing"
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
