//go:build acceptance

package acceptance

import (
	"strings"
	"testing"
)

func TestEnvironmentListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	// chef-zero auto-creates the "_default" environment in every org;
	// the seed adds "prod" and "staging" on top.
	human := runCinc(t, env.binary, "environment", "list", "--config", env.cfgPath)
	if human != "_default\nprod\nstaging\n" {
		t.Errorf("environment list (human) = %q", human)
	}

	jsonOut := runCinc(t, env.binary, "environment", "list", "--config", env.cfgPath, "--format", "json")
	for _, name := range []string{"_default", "prod", "staging"} {
		if !strings.Contains(jsonOut, name) {
			t.Errorf("environment list (json) missing %q\ngot: %s", name, jsonOut)
		}
	}
}

func TestEnvironmentDeleteAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := runCinc(t, env.binary, "environment", "delete", "staging", "--config", env.cfgPath)
	if out != "Deleted environment \"staging\"\n" {
		t.Errorf("environment delete output = %q", out)
	}

	after := runCinc(t, env.binary, "environment", "list", "--config", env.cfgPath)
	if after != "_default\nprod\n" {
		t.Errorf("environment list after delete = %q, want staging absent", after)
	}
}
