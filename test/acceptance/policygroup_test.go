//go:build acceptance

package acceptance

import (
	"strings"
	"testing"
)

// TestPolicyGroupListAgainstChefZero asserts an empty policy-group
// index returns an empty list in both formats. The chef-zero seed
// does not contain any policy groups.
func TestPolicyGroupListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "policy-group", "list", "--config", env.cfgPath)
	if human != "" {
		t.Errorf("policy-group list (human) = %q, want empty", human)
	}

	jsonOut := strings.TrimSpace(runCinc(t, env.binary, "policy-group", "list", "--config", env.cfgPath, "--format", "json"))
	if jsonOut != "[]" {
		t.Errorf("policy-group list (json) = %q, want \"[]\"", jsonOut)
	}
}
