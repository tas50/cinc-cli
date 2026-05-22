package cookbook

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestLoadMetadataJSONGeneratesFromMetadataRB(t *testing.T) {
	dir := t.TempDir()
	metadataRB := []byte(`
name 'sample'
maintainer 'Sous Chefs'
maintainer_email 'help@sous-chefs.org'
license 'Apache-2.0'
description 'Sample cookbook'
long_description 'Longer text'
version '1.2.3'
source_url 'https://example.test/source'
issues_url 'https://example.test/issues'
chef_version '>= 16'
ohai_version '>= 17'
supports :ubuntu, '>= 20.04'
supports 'debian'
depends 'apt', '~> 7.0'
provides 'sample::default'
recipe 'sample::default', 'Configures sample'
`)
	if err := os.WriteFile(filepath.Join(dir, "metadata.rb"), metadataRB, 0o644); err != nil {
		t.Fatal(err)
	}

	md, err := LoadMetadataJSON(dir)
	if err != nil {
		t.Fatalf("LoadMetadataJSON: %v", err)
	}
	if md.Name != "sample" || md.Version != "1.2.3" {
		t.Fatalf("metadata = %+v, want sample 1.2.3", md)
	}

	doc, err := LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if !doc.Generated {
		t.Fatal("metadata.rb-only cookbook did not report generated metadata")
	}
	var got map[string]any
	if err := json.Unmarshal(doc.JSON, &got); err != nil {
		t.Fatalf("generated metadata JSON: %v", err)
	}
	assertString(t, got, "description", "Sample cookbook")
	assertString(t, got, "long_description", "Longer text")
	assertNestedString(t, got, "platforms", "ubuntu", ">= 20.04")
	assertNestedString(t, got, "platforms", "debian", ">= 0.0.0")
	assertNestedString(t, got, "dependencies", "apt", "~> 7.0")
	assertNestedString(t, got, "providing", "sample::default", ">= 0.0.0")
	assertNestedString(t, got, "recipes", "sample::default", "Configures sample")
	assertConstraint(t, got, "chef_versions", ">= 16")
	assertConstraint(t, got, "ohai_versions", ">= 17")
	if got["privacy"] != false {
		t.Fatalf("privacy = %v, want false", got["privacy"])
	}
	if got["eager_load_libraries"] != true {
		t.Fatalf("eager_load_libraries = %v, want true", got["eager_load_libraries"])
	}
}

func TestBuildArchiveRootsFilesAtCookbookName(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"name":"nginx","version":"1.2.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recipes", "default.rb"), []byte("package 'nginx'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	archive, err := BuildArchive(dir, "nginx")
	if err != nil {
		t.Fatalf("BuildArchive: %v", err)
	}
	got, err := ExtractArchiveFiles(archive.Bytes)
	if err != nil {
		t.Fatalf("ExtractArchiveFiles: %v", err)
	}
	want := []string{"nginx/metadata.json", "nginx/recipes/default.rb"}
	if !slices.Equal(got, want) {
		t.Fatalf("archive files = %v, want %v", got, want)
	}
}

func TestReadVersionAcceptsLiteralMetadataRBVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "metadata.rb"), []byte("name 'nginx'\nversion '1.2.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	version, err := ReadVersion(dir)
	if err != nil {
		t.Fatalf("ReadVersion: %v", err)
	}
	if version != "1.2.0" {
		t.Fatalf("version = %q, want 1.2.0", version)
	}
}

func TestLoadMetadataReportsUnsupportedRubyExpressions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "metadata.rb"), []byte("name 'nginx'\nversion '1.2.0'\nplatform = 'ubuntu'\nsupports platform\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMetadata(dir)
	if err == nil {
		t.Fatal("expected unsupported Ruby expression error")
	}
	if !strings.Contains(err.Error(), `unsupported metadata.rb argument "platform"`) {
		t.Fatalf("error = %q, want unsupported argument", err)
	}
}

func TestBuildArchiveCanOverlayGeneratedMetadataJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.rb"), []byte("name 'nginx'\nversion '1.2.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recipes", "default.rb"), []byte("package 'nginx'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	archive, err := BuildArchiveWithMetadata(dir, "nginx", doc.JSON)
	if err != nil {
		t.Fatalf("BuildArchiveWithMetadata: %v", err)
	}
	got, err := ExtractArchiveFiles(archive.Bytes)
	if err != nil {
		t.Fatalf("ExtractArchiveFiles: %v", err)
	}
	want := []string{"nginx/metadata.json", "nginx/metadata.rb", "nginx/recipes/default.rb"}
	if !slices.Equal(got, want) {
		t.Fatalf("archive files = %v, want %v", got, want)
	}
	metadataJSON := archiveFile(t, archive.Bytes, "nginx/metadata.json")
	var md Metadata
	if err := json.Unmarshal(metadataJSON, &md); err != nil {
		t.Fatalf("archive metadata.json: %v", err)
	}
	if md.Name != "nginx" || md.Version != "1.2.0" {
		t.Fatalf("archive metadata = %+v, want nginx 1.2.0", md)
	}
}

func TestBuildArchiveRespectsChefignore(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"recipes", "spec/fixtures", ".kitchen"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"metadata.json":          `{"name":"nginx","version":"1.0.0"}`,
		"chefignore":             "*.bak\nspec/*\n.kitchen\nBerksfile.lock\n",
		"recipes/default.rb":     "package 'nginx'\n",
		"recipes/default.bak":    "old\n",
		"Berksfile.lock":         "lock\n",
		"spec/spec_helper.rb":    "# spec\n",
		"spec/fixtures/foo.json": "{}\n",
		".kitchen/state.yml":     "state\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	archive, err := BuildArchive(dir, "nginx")
	if err != nil {
		t.Fatalf("BuildArchive: %v", err)
	}
	got, err := ExtractArchiveFiles(archive.Bytes)
	if err != nil {
		t.Fatalf("ExtractArchiveFiles: %v", err)
	}
	want := []string{"nginx/chefignore", "nginx/metadata.json", "nginx/recipes/default.rb"}
	if !slices.Equal(got, want) {
		t.Fatalf("archive files = %v, want %v", got, want)
	}
}

func TestBuildArchiveSkipChefignoreIncludesEverything(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"metadata.json":       `{"name":"nginx","version":"1.0.0"}`,
		"chefignore":          "*.bak\n",
		"recipes/default.rb":  "package 'nginx'\n",
		"recipes/default.bak": "old\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	archive, err := BuildArchiveWithOptions(dir, "nginx", ArchiveOptions{
		IncludeFiles:   true,
		SkipChefignore: true,
	})
	if err != nil {
		t.Fatalf("BuildArchiveWithOptions: %v", err)
	}
	got, err := ExtractArchiveFiles(archive.Bytes)
	if err != nil {
		t.Fatalf("ExtractArchiveFiles: %v", err)
	}
	want := []string{"nginx/chefignore", "nginx/metadata.json", "nginx/recipes/default.bak", "nginx/recipes/default.rb"}
	if !slices.Equal(got, want) {
		t.Fatalf("archive files = %v, want %v", got, want)
	}
}

func BenchmarkBuildUploadArchiveWithLargeGitDirectory(b *testing.B) {
	dir := b.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "recipes"), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"name":"nginx","version":"1.2.0"}`), 0o644); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if err := os.WriteFile(filepath.Join(dir, "recipes", "recipe_"+strconv.Itoa(i)+".rb"), []byte("package 'nginx'\n"), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	for i := 0; i < 2000; i++ {
		path := filepath.Join(dir, ".git", "objects", strconv.Itoa(i%100), strconv.Itoa(i))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("ignored git object"), 0o644); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BuildUploadArchive(dir, "nginx"); err != nil {
			b.Fatal(err)
		}
	}
}

func assertString(t *testing.T, got map[string]any, key, want string) {
	t.Helper()
	if got[key] != want {
		t.Fatalf("%s = %v, want %q", key, got[key], want)
	}
}

func assertNestedString(t *testing.T, got map[string]any, key, nestedKey, want string) {
	t.Helper()
	nested, ok := got[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %T, want object", key, got[key])
	}
	if nested[nestedKey] != want {
		t.Fatalf("%s[%s] = %v, want %q", key, nestedKey, nested[nestedKey], want)
	}
}

func assertConstraint(t *testing.T, got map[string]any, key, want string) {
	t.Helper()
	constraints, ok := got[key].([]any)
	if !ok || len(constraints) != 1 {
		t.Fatalf("%s = %#v, want one constraint group", key, got[key])
	}
	group, ok := constraints[0].([]any)
	if !ok || len(group) != 1 || group[0] != want {
		t.Fatalf("%s[0] = %#v, want [%q]", key, constraints[0], want)
	}
}

func archiveFile(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name != name {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	t.Fatalf("archive missing %s", name)
	return nil
}
