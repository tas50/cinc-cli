package cookbook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocateFindsCookbookByNameUnderCurrentDir(t *testing.T) {
	tmp := t.TempDir()
	writeCookbookDir(t, filepath.Join(tmp, "nginx"))
	pushdir(t, tmp)

	dir, err := Locate("nginx", "")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if filepath.Base(dir) != "nginx" {
		t.Fatalf("located %q, want directory whose basename is nginx", dir)
	}
}

func TestLocateAcceptsCookbookPathAsList(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	writeCookbookDir(t, filepath.Join(second, "nginx"))

	dir, err := Locate("nginx", first+string(filepath.ListSeparator)+second)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if filepath.Dir(dir) != second {
		t.Fatalf("located %q, want it under second base %q", dir, second)
	}
}

func TestLocateAcceptsCookbookPathPointingAtCookbookItself(t *testing.T) {
	tmp := t.TempDir()
	cookbookDir := filepath.Join(tmp, "nginx")
	writeCookbookDir(t, cookbookDir)

	dir, err := Locate("nginx", cookbookDir)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if dir != cookbookDir {
		t.Fatalf("located %q, want %q", dir, cookbookDir)
	}
}

func TestLocateReportsMissingCookbookWithCurrentDirHint(t *testing.T) {
	pushdir(t, t.TempDir())

	_, err := Locate("nginx", "")
	if err == nil {
		t.Fatal("expected missing cookbook error")
	}
	if !strings.Contains(err.Error(), "current directory") {
		t.Fatalf("error = %q, want hint about current directory", err)
	}
}

func TestLocateReportsMissingCookbookWithExplicitPath(t *testing.T) {
	tmp := t.TempDir()

	_, err := Locate("nginx", tmp)
	if err == nil {
		t.Fatal("expected missing cookbook error")
	}
	if !strings.Contains(err.Error(), tmp) {
		t.Fatalf("error = %q, want explicit cookbook path in message", err)
	}
}

func TestLocateRejectsDirectoryMissingMetadata(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "nginx", "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Locate("nginx", tmp)
	if err == nil {
		t.Fatal("expected error when cookbook dir has no metadata file")
	}
}

func writeCookbookDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"name":"nginx","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func pushdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prev)
	})
}
