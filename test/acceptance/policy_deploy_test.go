//go:build acceptance

package acceptance

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
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

// git runs a git subcommand in dir, failing the test on error.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// TestPolicyPushGitSourceAgainstCincZero builds a local git repo holding a
// cookbook, writes a lock pinning it by git revision, and pushes it through the
// real binary (which clones the repo and uploads the artifact) to cinc-zero.
func TestPolicyPushGitSourceAgainstCincZero(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	env, stop := startAcceptance(t)
	defer stop()

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "metadata.rb"), []byte("name 'gitcb'\nversion '1.0.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "recipes", "default.rb"), []byte("log 'git'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "--quiet")
	git(t, repo, "config", "user.email", "t@example.test")
	git(t, repo, "config", "user.name", "Test")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "--quiet", "-m", "cookbook")
	sha := strings.TrimSpace(git(t, repo, "rev-parse", "HEAD"))

	lockDir := t.TempDir()
	const identifier = "1111111111111111111111111111111111111111"
	lock := map[string]any{
		"name":        "gitpolicy",
		"revision_id": identifier,
		"run_list":    []string{"recipe[gitcb]"},
		"cookbook_locks": map[string]any{
			"gitcb": map[string]any{
				"version":                   "1.0.0",
				"identifier":                identifier,
				"dotted_decimal_identifier": "1.2.3",
				"cache_key":                 "gitcb-1.0.0-git",
				"source_options":            map[string]any{"git": repo, "revision": sha},
			},
		},
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	lockPath := filepath.Join(lockDir, "Policyfile.lock.json")
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	out := runCinc(t, env.binary, "policy", "push", "gitqa", lockPath, "--config", env.cfgPath)
	if !strings.Contains(out, "Pushed policy \"gitpolicy\"") {
		t.Fatalf("git-source push output = %q", out)
	}
	list := runCinc(t, env.binary, "policy", "list", "--config", env.cfgPath)
	if !strings.Contains(list, "gitpolicy") {
		t.Errorf("policy list after git push missing gitpolicy:\n%s", list)
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

// TestPolicyPushArchiveAgainstCincZero exports path-sourced locks to bundles and
// deploys both the directory and the .tar.gz form with push-archive, each to a
// fresh policy group, confirming the policy and its group association land on the
// server — the full export -> push-archive round trip.
//
// The two pushes use distinct policies/identifiers because cinc-zero rejects
// re-uploading an existing cookbook-artifact identifier with 409 (a real Chef
// server treats identical-content uploads as idempotent), so pushing the same
// bundle twice would collide.
func TestPolicyPushArchiveAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	// Directory form -> group "archiveqa".
	dirLock := writeDeployFixture(t, "archivetest", "fedcba9876543210fedcba9876543210fedcba98")
	dirBundle := filepath.Join(t.TempDir(), "archivebundle")
	if out := runCinc(t, env.binary, "policy", "export", dirLock, dirBundle, "--config", env.cfgPath); !strings.Contains(out, "Exported policy \"archivetest\"") {
		t.Fatalf("policy export (dir) output = %q", out)
	}
	out := runCinc(t, env.binary, "policy", "push-archive", "archiveqa", dirBundle, "--config", env.cfgPath)
	if !strings.Contains(out, "Pushed policy \"archivetest\"") || !strings.Contains(out, "group \"archiveqa\"") {
		t.Fatalf("push-archive (dir) output = %q", out)
	}
	if list := runCinc(t, env.binary, "policy", "list", "--config", env.cfgPath); !strings.Contains(list, "archivetest") {
		t.Errorf("policy list after push-archive missing archivetest:\n%s", list)
	}
	if group := runCinc(t, env.binary, "policy-group", "show", "archiveqa", "--config", env.cfgPath, "--format", "json"); !strings.Contains(group, "archivetest") {
		t.Errorf("policy-group show archiveqa missing archivetest:\n%s", group)
	}

	// Tarball form (a different policy + identifier) -> group "archiveqa2".
	tarLock := writeDeployFixture(t, "archivetar", "0011223344556677889900112233445566778899")
	tarBundle := filepath.Join(t.TempDir(), "archivetarbundle")
	if out := runCinc(t, env.binary, "policy", "export", tarLock, tarBundle, "--archive", "--config", env.cfgPath); !strings.Contains(out, "Exported policy \"archivetar\"") {
		t.Fatalf("policy export (tarball) output = %q", out)
	}
	out = runCinc(t, env.binary, "policy", "push-archive", "archiveqa2", tarBundle+".tar.gz", "--config", env.cfgPath)
	if !strings.Contains(out, "Pushed policy \"archivetar\"") || !strings.Contains(out, "group \"archiveqa2\"") {
		t.Fatalf("push-archive (tarball) output = %q", out)
	}
	if group := runCinc(t, env.binary, "policy-group", "show", "archiveqa2", "--config", env.cfgPath, "--format", "json"); !strings.Contains(group, "archivetar") {
		t.Errorf("policy-group show archiveqa2 missing archivetar:\n%s", group)
	}
}
