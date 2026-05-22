package cookbook

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseChefignoreSkipsCommentsAndBlanks(t *testing.T) {
	patterns := parseChefignore([]byte("# a comment\n\n*.bak\n  spec/* \n\n# another\nBerksfile.lock\n"))
	want := []string{"*.bak", "spec/*", "Berksfile.lock"}
	if !slices.Equal(patterns, want) {
		t.Fatalf("patterns = %v, want %v", patterns, want)
	}
}

func TestLoadChefignoreReturnsNilWhenMissing(t *testing.T) {
	patterns, err := LoadChefignore(t.TempDir())
	if err != nil {
		t.Fatalf("LoadChefignore: %v", err)
	}
	if patterns != nil {
		t.Fatalf("patterns = %v, want nil for missing file", patterns)
	}
}

func TestLoadChefignoreReadsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chefignore"), []byte("*.bak\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	patterns, err := LoadChefignore(dir)
	if err != nil {
		t.Fatalf("LoadChefignore: %v", err)
	}
	if !slices.Equal(patterns, []string{"*.bak"}) {
		t.Fatalf("patterns = %v, want [*.bak]", patterns)
	}
}

func TestChefignoreMatchesByBasename(t *testing.T) {
	patterns := []string{"*.bak"}
	cases := map[string]bool{
		"recipes/default.rb":  false,
		"recipes/default.bak": true,
		"deep/nested/foo.bak": true,
		"foo.bak":             true,
		"foobak":              false,
	}
	for relPath, want := range cases {
		if got := chefignoreMatches(patterns, relPath); got != want {
			t.Errorf("%q: got %v, want %v", relPath, got, want)
		}
	}
}

func TestChefignoreMatchesByDirectorySegment(t *testing.T) {
	patterns := []string{"spec/*", ".kitchen"}
	cases := map[string]bool{
		"spec/foo_spec.rb":        true,
		"spec/fixtures/sample.rb": true,
		".kitchen/state.yml":      true,
		".kitchen":                true,
		"recipes/default.rb":      false,
		"libraries/helper.rb":     false,
	}
	for relPath, want := range cases {
		if got := chefignoreMatches(patterns, relPath); got != want {
			t.Errorf("%q: got %v, want %v", relPath, got, want)
		}
	}
}

func TestChefignoreMatchesFullPath(t *testing.T) {
	patterns := []string{"recipes/default.bak"}
	if !chefignoreMatches(patterns, "recipes/default.bak") {
		t.Fatal("expected full-path match")
	}
	if chefignoreMatches(patterns, "recipes/other.bak") {
		t.Fatal("did not expect match for a different file")
	}
}

func TestChefignoreEmptyPatternsMatchesNothing(t *testing.T) {
	if chefignoreMatches(nil, "anything.rb") {
		t.Fatal("nil patterns should not match")
	}
	if chefignoreMatches([]string{}, "anything.rb") {
		t.Fatal("empty patterns should not match")
	}
}
