//go:build acceptance

package acceptance

import (
	"strings"
	"testing"
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
