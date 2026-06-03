package cookbook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMetadataReadsMetadataJSON(t *testing.T) {
	dir := t.TempDir()
	body := []byte(`{"name":"nginx","version":"1.2.0"}`)
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if doc.Generated {
		t.Fatal("metadata.json should not be reported as generated")
	}
	if doc.Metadata.Name != "nginx" || doc.Metadata.Version != "1.2.0" {
		t.Fatalf("metadata = %+v, want nginx 1.2.0", doc.Metadata)
	}
	if string(doc.JSON) != string(body) {
		t.Fatalf("JSON = %q, want raw file bytes", doc.JSON)
	}
}

func TestLoadMetadataRejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMetadata(dir)
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
	if !strings.Contains(err.Error(), "metadata.json") {
		t.Fatalf("error = %q, want metadata.json in message", err)
	}
}

func TestLoadMetadataRequiresName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"version":"1.2.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMetadata(dir)
	if err == nil || !strings.Contains(err.Error(), "missing name") {
		t.Fatalf("error = %v, want missing name", err)
	}
}

func TestLoadMetadataRequiresVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"name":"nginx"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMetadata(dir)
	if err == nil || !strings.Contains(err.Error(), "missing version") {
		t.Fatalf("error = %v, want missing version", err)
	}
}

func TestLoadMetadataReportsMissingFiles(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadMetadata(dir)
	if err == nil {
		t.Fatal("expected missing metadata error")
	}
	if !strings.Contains(err.Error(), "metadata.json") {
		t.Fatalf("error = %q, want metadata.json in message", err)
	}
}

func TestReadVersionPrefersMetadataJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"name":"nginx","version":"2.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.rb"), []byte("name 'nginx'\nversion '9.9.9'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadVersion(dir)
	if err != nil {
		t.Fatalf("ReadVersion: %v", err)
	}
	if got != "2.0.0" {
		t.Fatalf("version = %q, want metadata.json version 2.0.0", got)
	}
}

func TestReadVersionErrorsWhenMetadataAbsent(t *testing.T) {
	_, err := ReadVersion(t.TempDir())
	if err == nil {
		t.Fatal("expected missing-metadata error")
	}
}
