//go:build acceptance

package acceptance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCookbookListAgainstCincZero asserts the seeded `webserver` cookbook shows
// up in the list in both formats. (Before a cookbook was seeded this asserted an
// empty list; the seed now carries one so the explorer has real metadata to read.)
func TestCookbookListAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "cookbook", "list", "--config", env.cfgPath)
	if !strings.Contains(human, "webserver") {
		t.Errorf("cookbook list (human) = %q, want it to contain webserver", human)
	}

	jsonOut := runCinc(t, env.binary, "cookbook", "list", "--config", env.cfgPath, "--format", "json")
	var names []string
	if err := json.Unmarshal([]byte(jsonOut), &names); err != nil {
		t.Fatalf("cookbook list (json) is not valid JSON: %v\noutput: %s", err, jsonOut)
	}
	if !slices.Contains(names, "webserver") {
		t.Errorf("cookbook list (json) = %v, want it to contain webserver", names)
	}
}

func TestCookbookUploadDeleteAgainstCincZero(t *testing.T) {
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

	showOut := runCinc(t, env.binary, "cookbook", "show", "nginx", "1.2.0", "--config", env.cfgPath, "--format", "json")
	var showManifest struct {
		CookbookName string `json:"cookbook_name"`
		Version      string `json:"version"`
	}
	if err := json.Unmarshal([]byte(showOut), &showManifest); err != nil {
		t.Fatalf("cookbook show output is not valid JSON: %v\noutput: %s", err, showOut)
	}
	if showManifest.CookbookName != "nginx" || showManifest.Version != "1.2.0" {
		t.Errorf("cookbook show returned %+v, want cookbook_name=nginx version=1.2.0", showManifest)
	}

	latestOut := runCinc(t, env.binary, "cookbook", "show", "nginx", "--config", env.cfgPath, "--format", "json")
	var latestManifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(latestOut), &latestManifest); err != nil {
		t.Fatalf("cookbook show (latest) output is not valid JSON: %v\noutput: %s", err, latestOut)
	}
	if latestManifest.Version != "1.2.0" {
		t.Errorf("cookbook show (latest) version = %q, want 1.2.0", latestManifest.Version)
	}

	deleteOut := runCinc(t, env.binary, "cookbook", "delete", "nginx", "1.2.0", "--config", env.cfgPath)
	if deleteOut != "Deleted cookbook \"nginx\" version 1.2.0\n" {
		t.Errorf("cookbook delete output = %q", deleteOut)
	}
}

// TestCookbookShowMissingAgainstCincZero asserts that asking for a
// cookbook the server has never seen surfaces the server's 404.
func TestCookbookShowMissingAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	_, stderr, err := runCincRaw(env.binary, "cookbook", "show", "ghost", "--config", env.cfgPath)
	if err == nil {
		t.Fatalf("cookbook show of missing cookbook unexpectedly succeeded")
	}
	if !strings.Contains(stderr, "404") && !strings.Contains(stderr, "not found") {
		t.Errorf("cookbook show stderr does not mention 404/not found: %s", stderr)
	}
}

// TestCookbookDeleteMissingAgainstCincZero exercises the delete code
// path against a real server when the cookbook does not exist. The
// command must exit non-zero and surface the server's 404.
func TestCookbookDeleteMissingAgainstCincZero(t *testing.T) {
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

// TestCookbookShowSurfacesMetadataAgainstCincZero confirms cinc-zero serves the
// identity metadata a cookbook declares in metadata.rb — description, maintainer
// and contact, license, project URLs, and dependencies — and that `cinc cookbook
// show` carries it through. This is the data source the explorer's cookbook-
// version summary pane reads; the pane itself can't be exercised here because the
// TUI refuses a non-TTY stdout (see TestExploreRequiresTTY), so the pane
// rendering is covered by the unit tests in cli/explore.
//
// cinc-zero only parses this metadata on the --repo seed-load path, so the
// assertion runs against the seeded `webserver` cookbook rather than an upload
// (the upload path stores the client-computed manifest untouched). The
// maintainer, maintainer_email, source_url, and issues_url fields are served as
// of cinc-zero v0.6.3 (PR #65).
func TestCookbookShowSurfacesMetadataAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	showOut := runCinc(t, env.binary, "cookbook", "show", "webserver", "2.1.0", "--config", env.cfgPath, "--format", "json")
	var manifest struct {
		Metadata struct {
			Description     string            `json:"description"`
			Maintainer      string            `json:"maintainer"`
			MaintainerEmail string            `json:"maintainer_email"`
			License         string            `json:"license"`
			SourceURL       string            `json:"source_url"`
			IssuesURL       string            `json:"issues_url"`
			Dependencies    map[string]string `json:"dependencies"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(showOut), &manifest); err != nil {
		t.Fatalf("cookbook show output is not valid JSON: %v\noutput: %s", err, showOut)
	}
	md := manifest.Metadata
	for _, c := range []struct{ field, got, want string }{
		{"description", md.Description, "Installs and configures the acme web server"},
		{"maintainer", md.Maintainer, "Acme Infra"},
		{"maintainer_email", md.MaintainerEmail, "infra@acme.test"},
		{"license", md.License, "Apache-2.0"},
		{"source_url", md.SourceURL, "https://github.com/acme/webserver"},
		{"issues_url", md.IssuesURL, "https://github.com/acme/webserver/issues"},
	} {
		if c.got != c.want {
			t.Errorf("metadata.%s = %q, want %q", c.field, c.got, c.want)
		}
	}
	if got := md.Dependencies["apt"]; got != ">= 7.0" {
		t.Errorf("metadata.dependencies[apt] = %q, want >= 7.0", got)
	}
	if _, ok := md.Dependencies["build-essential"]; !ok {
		t.Errorf("metadata.dependencies missing build-essential: %+v", md.Dependencies)
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
