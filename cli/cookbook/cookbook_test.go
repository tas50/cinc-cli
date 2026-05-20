package cookbook

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadMetadataJSONRequiresJSONForSupermarket(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "metadata.rb"), []byte("name 'nginx'\nversion '1.2.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMetadataJSON(dir)
	if err == nil {
		t.Fatal("expected error for metadata.rb-only cookbook")
	}
	if !strings.Contains(err.Error(), "metadata.json is required") {
		t.Fatalf("error = %q, want metadata.json guidance", err)
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
