//go:build acceptance

package acceptance

import (
	"strings"
	"testing"
)

// TestPolicyListAgainstChefZero asserts an empty policy index returns
// an empty list in both formats. The chef-zero seed does not contain
// any policies, so the endpoint must respond cleanly with an empty
// result.
func TestPolicyListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "policy", "list", "--config", env.cfgPath)
	if human != "" {
		t.Errorf("policy list (human) = %q, want empty", human)
	}

	jsonOut := strings.TrimSpace(runCinc(t, env.binary, "policy", "list", "--config", env.cfgPath, "--format", "json"))
	if jsonOut != "[]" {
		t.Errorf("policy list (json) = %q, want \"[]\"", jsonOut)
	}
}
