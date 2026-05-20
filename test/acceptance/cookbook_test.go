//go:build acceptance

package acceptance

import (
	"strings"
	"testing"
)

// TestCookbookListAgainstChefZero asserts that an empty server returns
// an empty list in both formats. Cookbooks are not seeded because
// chef-zero needs a full version manifest per cookbook and there is no
// `cinc cookbook upload` command yet to populate one from the CLI.
func TestCookbookListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "cookbook", "list", "--config", env.cfgPath)
	if human != "" {
		t.Errorf("cookbook list (human) = %q, want empty", human)
	}

	jsonOut := strings.TrimSpace(runCinc(t, env.binary, "cookbook", "list", "--config", env.cfgPath, "--format", "json"))
	if jsonOut != "[]" {
		t.Errorf("cookbook list (json) = %q, want \"[]\"", jsonOut)
	}
}

// TestCookbookDeleteMissingAgainstChefZero exercises the delete code
// path against a real server when the cookbook does not exist. The
// command must exit non-zero and surface the server's 404. Once cinc
// gains an upload verb this should be expanded to seed a cookbook,
// delete it, and assert it disappears from the list.
func TestCookbookDeleteMissingAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	_, stderr, err := runCincRaw(env.binary, "cookbook", "delete", "ghost", "0.0.1", "--config", env.cfgPath)
	if err == nil {
		t.Fatalf("cookbook delete of missing cookbook unexpectedly succeeded")
	}
	if !strings.Contains(stderr, "404") && !strings.Contains(stderr, "not found") {
		t.Errorf("cookbook delete stderr does not mention 404/not found: %s", stderr)
	}
}
