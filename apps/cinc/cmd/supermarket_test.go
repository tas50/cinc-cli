package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupermarketShareDryRunCommand(t *testing.T) {
	cookbookPath := writeCommandSupermarketCookbook(t, "nginx")
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"supermarket", "share", "nginx", "Other",
		"--dry-run", "--cookbook-path", cookbookPath,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc supermarket share --dry-run: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "Making tarball nginx.tgz\n") {
		t.Fatalf("output = %q, want tarball line", got)
	}
	if !strings.Contains(got, "nginx/metadata.json\n") {
		t.Fatalf("output = %q, want archive file list", got)
	}
}

func TestSupermarketShareDryRunCommandJSON(t *testing.T) {
	cookbookPath := writeCommandSupermarketCookbook(t, "nginx")
	cfgPath := filepath.Join(t.TempDir(), "credentials")
	cfg := fmt.Sprintf(`[default]
cinc_server_url = "https://chef.example.test/organizations/acme"
client_name     = "tim"
client_key      = %q
`, writeTestKey(t))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"supermarket", "share", "nginx", "Other",
		"--dry-run", "--cookbook-path", cookbookPath,
		"--format", "json", "--config", cfgPath,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc supermarket share --dry-run --format json: %v", err)
	}
	var result struct {
		Cookbook string `json:"cookbook"`
		Category string `json:"category"`
		Uploaded bool   `json:"uploaded"`
		Status   int    `json:"status"`
		Tarball  string `json:"tarball"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, buf.String())
	}
	if result.Cookbook != "nginx" || result.Category != "Other" || result.Uploaded || result.Status != 0 || result.Tarball != "nginx.tgz" {
		t.Fatalf("result = %+v", result)
	}
}

func TestSupermarketShareDryRunFiltersChefignoreByDefault(t *testing.T) {
	root := writeCommandSupermarketCookbookWithChefignore(t, "nginx")
	cmdRoot := newRootCmd()
	var buf bytes.Buffer
	cmdRoot.SetOut(&buf)
	cmdRoot.SetArgs([]string{
		"supermarket", "share", "nginx", "Other",
		"--dry-run", "--cookbook-path", root,
	})

	if err := cmdRoot.Execute(); err != nil {
		t.Fatalf("cinc supermarket share --dry-run: %v", err)
	}
	if strings.Contains(buf.String(), "default.bak") {
		t.Fatalf("output includes chefignored file:\n%s", buf.String())
	}
}

func TestSupermarketShareDryRunNoChefignoreIncludesIgnoredFiles(t *testing.T) {
	root := writeCommandSupermarketCookbookWithChefignore(t, "nginx")
	cmdRoot := newRootCmd()
	var buf bytes.Buffer
	cmdRoot.SetOut(&buf)
	cmdRoot.SetArgs([]string{
		"supermarket", "share", "nginx", "Other",
		"--dry-run", "--no-chefignore", "--cookbook-path", root,
	})

	if err := cmdRoot.Execute(); err != nil {
		t.Fatalf("cinc supermarket share --dry-run --no-chefignore: %v", err)
	}
	if !strings.Contains(buf.String(), "default.bak") {
		t.Fatalf("--no-chefignore did not include ignored file:\n%s", buf.String())
	}
}

func writeCommandSupermarketCookbookWithChefignore(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"metadata.json":       `{"name":"` + name + `","version":"1.2.0"}`,
		"chefignore":          "*.bak\n",
		"recipes/default.rb":  "package 'nginx'\n",
		"recipes/default.bak": "old\n",
	}
	for sub, body := range files {
		if err := os.WriteFile(filepath.Join(dir, sub), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeCommandSupermarketCookbook(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"name":"`+name+`","version":"1.2.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recipes", "default.rb"), []byte("package 'nginx'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
