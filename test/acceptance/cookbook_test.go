//go:build acceptance

package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCookbookListAgainstChefZero asserts that an empty server returns
// an empty list in both formats.
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

func TestCookbookUploadDeleteAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	cookbookPath := writeAcceptanceCookbook(t, "nginx")
	upload := runCinc(t, env.binary, "cookbook", "upload", "nginx", "--cookbook-path", cookbookPath, "--config", env.cfgPath)
	if upload != "Uploaded cookbook \"nginx\" version 1.2.0\n" {
		t.Errorf("cookbook upload output = %q", upload)
	}

	list := runCinc(t, env.binary, "cookbook", "list", "--config", env.cfgPath)
	if !strings.Contains(list, "nginx\n") {
		t.Fatalf("cookbook list after upload = %q, want nginx", list)
	}

	deleteOut := runCinc(t, env.binary, "cookbook", "delete", "nginx", "1.2.0", "--config", env.cfgPath)
	if deleteOut != "Deleted cookbook \"nginx\" version 1.2.0\n" {
		t.Errorf("cookbook delete output = %q", deleteOut)
	}
}

// TestCookbookDeleteMissingAgainstChefZero exercises the delete code
// path against a real server when the cookbook does not exist. The
// command must exit non-zero and surface the server's 404.
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

func writeAcceptanceCookbook(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.rb"), []byte("name '"+name+"'\nversion '1.2.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recipes", "default.rb"), []byte("package 'nginx'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
