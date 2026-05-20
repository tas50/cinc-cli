//go:build acceptance

package acceptance

import (
	"strings"
	"testing"
)

// TestNodeListAgainstChefZero verifies that `cinc node list` against a
// real chef-zero server returns the seeded nodes in both human and
// JSON output formats.
func TestNodeListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "node", "list", "--config", env.cfgPath)
	if human != "db01\nweb01\nweb02\n" {
		t.Errorf("node list (human) = %q, want sorted node names", human)
	}

	jsonOut := runCinc(t, env.binary, "node", "list", "--config", env.cfgPath, "--format", "json")
	for _, name := range []string{"db01", "web01", "web02"} {
		if !strings.Contains(jsonOut, name) {
			t.Errorf("node list (json) missing %q\ngot: %s", name, jsonOut)
		}
	}
}

// TestNodeDeleteAgainstChefZero deletes one of the seeded nodes and
// verifies a follow-up list no longer includes it.
func TestNodeDeleteAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := runCinc(t, env.binary, "node", "delete", "web02", "--config", env.cfgPath)
	if out != "Deleted node \"web02\"\n" {
		t.Errorf("node delete output = %q", out)
	}

	after := runCinc(t, env.binary, "node", "list", "--config", env.cfgPath)
	if after != "db01\nweb01\n" {
		t.Errorf("node list after delete = %q, want web02 absent", after)
	}
}
