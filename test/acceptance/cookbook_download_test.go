//go:build acceptance

package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCookbookDownloadAgainstCincZero uploads a cookbook, downloads it back,
// and confirms the files land under <dir>/<name>-<version>/. With no version
// argument the "_latest" sentinel resolves to the only uploaded version.
func TestCookbookDownloadAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	cookbookPath := writeAcceptanceCookbook(t, "nginx")
	runCinc(t, env.binary, "cookbook", "upload", "nginx", "--cookbook-path", cookbookPath, "--config", env.cfgPath)

	destParent := t.TempDir()
	out := runCinc(t, env.binary, "cookbook", "download", "nginx", "--dir", destParent, "--config", env.cfgPath)
	if !strings.Contains(out, "nginx") || !strings.Contains(out, "1.2.0") {
		t.Errorf("download output = %q, want name and resolved version", out)
	}

	cbDir := filepath.Join(destParent, "nginx-1.2.0")
	recipe, err := os.ReadFile(filepath.Join(cbDir, "recipes", "default.rb"))
	if err != nil {
		t.Fatalf("downloaded recipe missing: %v", err)
	}
	if string(recipe) != "package 'nginx'\n" {
		t.Errorf("downloaded recipe content = %q", recipe)
	}
	if _, err := os.Stat(filepath.Join(cbDir, "metadata.rb")); err != nil {
		t.Errorf("downloaded metadata.rb missing: %v", err)
	}
}
