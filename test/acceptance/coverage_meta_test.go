//go:build acceptance

package acceptance

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/tas50/cinc-cli/apps/cinc/cmd"
)

// This meta-test is the durable coverage guard. It walks the live cobra
// command tree and the coverage manifest side by side and fails the moment
// they drift apart, so a new leaf command can't ship without either an
// acceptance test or a justified exemption.
//
// How a developer satisfies it after adding a `cinc <noun> <verb>` leaf:
//   1. Write the acceptance test (see CONVENTIONS in CLAUDE.md).
//   2. Add a [[command]] block to coverage_manifest.toml with
//      status = "covered" and tests = ["TestYourNewFunc"], OR
//      status = "exempt" with a reason if it genuinely can't run against
//      cinc-zero (external service, interactive TUI, …).
// Running `go test -tags acceptance ./test/acceptance/ -run TestCoverageManifest`
// then tells you exactly what's missing or stale.

// manifestEntry is one leaf-command record in coverage_manifest.toml.
type manifestEntry struct {
	Path   string   `toml:"path"`
	Status string   `toml:"status"` // "covered" or "exempt"
	Tests  []string `toml:"tests"`  // required when covered
	Reason string   `toml:"reason"` // required when exempt
}

type coverageManifest struct {
	Command []manifestEntry `toml:"command"`
}

// liveLeafCommands walks cmd.NewRootCmd() and returns every leaf command
// path (space-joined, sans the root "cinc"), skipping cobra's generated
// help/completion commands.
func liveLeafCommands() []string {
	var leaves []string
	var walk func(c *cobra.Command, path []string)
	walk = func(c *cobra.Command, path []string) {
		if c.Name() == "help" || c.Name() == "completion" {
			return
		}
		p := append(append([]string{}, path...), c.Name())
		var children []*cobra.Command
		for _, ch := range c.Commands() {
			if ch.Name() == "help" || ch.Name() == "completion" {
				continue
			}
			children = append(children, ch)
		}
		if len(children) == 0 {
			leaves = append(leaves, strings.Join(p[1:], " "))
			return
		}
		for _, ch := range children {
			walk(ch, p)
		}
	}
	walk(cmd.NewRootCmd(), nil)
	sort.Strings(leaves)
	return leaves
}

// acceptanceTestFuncs greps every test/acceptance/*_test.go file for
// top-level `func TestXxx(` declarations and returns the set of names.
func acceptanceTestFuncs(t *testing.T) map[string]bool {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the acceptance test source directory")
	}
	dir := filepath.Dir(thisFile)
	matches, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	funcRE := regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\s*\(`)
	funcs := map[string]bool{}
	for _, f := range matches {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range funcRE.FindAllStringSubmatch(string(src), -1) {
			funcs[m[1]] = true
		}
	}
	return funcs
}

// TestCoverageManifestMatchesCommandTree is the guard described above.
func TestCoverageManifestMatchesCommandTree(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the acceptance test source directory")
	}
	manifestPath := filepath.Join(filepath.Dir(thisFile), "coverage_manifest.toml")

	var manifest coverageManifest
	if _, err := toml.DecodeFile(manifestPath, &manifest); err != nil {
		t.Fatalf("reading %s: %v", manifestPath, err)
	}

	// Index the manifest by path, catching duplicates.
	byPath := make(map[string]manifestEntry, len(manifest.Command))
	for _, e := range manifest.Command {
		if _, dup := byPath[e.Path]; dup {
			t.Errorf("coverage_manifest.toml lists %q more than once", e.Path)
		}
		byPath[e.Path] = e
	}

	live := liveLeafCommands()
	liveSet := make(map[string]bool, len(live))
	for _, l := range live {
		liveSet[l] = true
	}

	// (a) Every live leaf must be in the manifest.
	for _, leaf := range live {
		if _, ok := byPath[leaf]; !ok {
			t.Errorf("leaf command %q is missing from coverage_manifest.toml — add it as covered (with tests) or exempt (with a reason)", leaf)
		}
	}

	// (b) Every manifest entry must map to a live leaf (no stale entries).
	for _, e := range manifest.Command {
		if !liveSet[e.Path] {
			t.Errorf("coverage_manifest.toml entry %q does not match any live leaf command — remove it or fix the path", e.Path)
		}
	}

	testFuncs := acceptanceTestFuncs(t)

	for _, e := range manifest.Command {
		switch e.Status {
		case "covered":
			// (d) Every named test must actually exist.
			if len(e.Tests) == 0 {
				t.Errorf("%q is marked covered but lists no tests", e.Path)
			}
			for _, tn := range e.Tests {
				if !testFuncs[tn] {
					t.Errorf("%q references acceptance test %q, which does not exist in test/acceptance/*_test.go", e.Path, tn)
				}
			}
		case "exempt":
			// (c) Exemptions must justify themselves.
			if strings.TrimSpace(e.Reason) == "" {
				t.Errorf("%q is marked exempt but has an empty reason", e.Path)
			}
		default:
			t.Errorf("%q has unknown status %q — want \"covered\" or \"exempt\"", e.Path, e.Status)
		}
	}
}
