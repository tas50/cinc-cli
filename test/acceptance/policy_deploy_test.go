//go:build acceptance

package acceptance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDeployFixture creates a lock dir with a path-sourced cookbook and a
// Policyfile.lock.json that pins it, returning the lock path.
func writeDeployFixture(t *testing.T, policy, identifier string) string {
	t.Helper()
	dir := t.TempDir()
	cb := filepath.Join(dir, "cookbooks", "deploycb", "recipes")
	if err := os.MkdirAll(cb, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cookbooks", "deploycb", "metadata.rb"), []byte("name 'deploycb'\nversion '1.0.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cb, "default.rb"), []byte("log 'deployed'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := map[string]any{
		"name":        policy,
		"revision_id": identifier,
		"run_list":    []string{"recipe[deploycb]"},
		"cookbook_locks": map[string]any{
			"deploycb": map[string]any{
				"version":                   "1.0.0",
				"identifier":                identifier,
				"dotted_decimal_identifier": "1.2.3",
				"source_options":            map[string]any{"path": "cookbooks/deploycb"},
			},
		},
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	lockPath := filepath.Join(dir, "Policyfile.lock.json")
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return lockPath
}

// TestPolicyPushAgainstCincZero pushes a path-sourced lock to a fresh policy
// group, uploading the cookbook artifact, then confirms the policy and its
// group association landed on the server.
func TestPolicyPushAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	const identifier = "0123456789abcdef0123456789abcdef01234567"
	lockPath := writeDeployFixture(t, "deploytest", identifier)

	out := runCinc(t, env.binary, "policy", "push", "qa", lockPath, "--config", env.cfgPath)
	if !strings.Contains(out, "Pushed policy \"deploytest\"") || !strings.Contains(out, "group \"qa\"") {
		t.Fatalf("policy push output = %q", out)
	}

	list := runCinc(t, env.binary, "policy", "list", "--config", env.cfgPath)
	if !strings.Contains(list, "deploytest") {
		t.Errorf("policy list after push missing deploytest:\n%s", list)
	}

	group := runCinc(t, env.binary, "policy-group", "show", "qa", "--config", env.cfgPath, "--format", "json")
	if !strings.Contains(group, "deploytest") {
		t.Errorf("policy-group show qa missing deploytest:\n%s", group)
	}
}

// TestPolicyExportAgainstCincZero assembles a bundle from a path-sourced lock
// (no server interaction needed for the export itself).
func TestPolicyExportAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	lockPath := writeDeployFixture(t, "exporttest", "abcdef0123456789abcdef0123456789abcdef01")
	destDir := filepath.Join(t.TempDir(), "bundle")

	out := runCinc(t, env.binary, "policy", "export", lockPath, destDir, "--archive", "--config", env.cfgPath)
	if !strings.Contains(out, "Exported policy \"exporttest\"") {
		t.Fatalf("policy export output = %q", out)
	}
	if _, err := os.Stat(filepath.Join(destDir, "cookbooks", "deploycb-1.2.3", "metadata.rb")); err != nil {
		t.Errorf("exported cookbook missing: %v", err)
	}
	if _, err := os.Stat(destDir + ".tar.gz"); err != nil {
		t.Errorf("archive missing: %v", err)
	}
}
