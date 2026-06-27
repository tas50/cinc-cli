package rubyeval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// divergentErrors lists fixtures whose chef error text the shim deliberately
// does not reproduce verbatim (chef emits a verbose backtrace / multi-line
// conflict message; the shim records a concise equivalent). For these we assert
// the error COUNT matches chef and that a meaningful substring is present,
// rather than byte-for-byte text. Every other fixture is compared exactly,
// errors included.
var divergentErrors = map[string]string{
	"err_raise":             `boom: something went wrong`,
	"err_syntax":            `syntax error`,
	"err_cookbook_conflict": `conflicting sources`,
}

// TestCorpus is the chef-compatibility evidence: it runs every Policyfile.rb
// fixture under testdata/ through the real embedded CRuby engine and diffs the
// result against a golden produced by chef-cli's own DSL (see
// generate_goldens.rb). It runs by default in `go test ./...`; if the pinned
// ruby.wasm cannot be fetched (offline CI) it skips with a clear message.
func TestCorpus(t *testing.T) {
	engine := NewEngine()
	if err := engine.Available(); err != nil {
		if IsUnavailable(err) {
			t.Skipf("skipping Policyfile corpus: ruby.wasm runtime unavailable (set up network access to run): %v", err)
		}
		t.Fatalf("unexpected error checking ruby.wasm availability: %v", err)
	}

	dirs, err := filepath.Glob(filepath.Join("testdata", "*", "Policyfile.rb"))
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Fatal("no fixtures found under testdata/")
	}

	for _, pf := range dirs {
		dir := filepath.Dir(pf)
		name := filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(pf)
			if err != nil {
				t.Fatal(err)
			}
			goldenBytes, err := os.ReadFile(filepath.Join(dir, "expected.json"))
			if err != nil {
				t.Fatalf("missing golden (run generate_goldens.rb): %v", err)
			}
			var want EvaluatedPolicy
			if err := json.Unmarshal(goldenBytes, &want); err != nil {
				t.Fatalf("decode golden: %v", err)
			}

			got, evalErr := engine.Evaluate(context.Background(), string(source), Options{
				Filename: "Policyfile.rb",
				Dir:      dir,
			})
			if got == nil {
				t.Fatalf("engine returned nil policy (err: %v)", evalErr)
			}

			// Reconcile error fields per the comparison policy, then diff the
			// whole structure for a single readable failure.
			if substr, divergent := divergentErrors[name]; divergent {
				if len(got.Errors) != len(want.Errors) {
					t.Errorf("error count: got %d %q, want %d %q", len(got.Errors), got.Errors, len(want.Errors), want.Errors)
				}
				if !containsAny(got.Errors, substr) {
					t.Errorf("expected an error containing %q, got %q", substr, got.Errors)
				}
				got.Errors = want.Errors // accept divergent text for the structural diff
			}

			gotJSON := mustCanonicalJSON(t, got)
			wantJSON := mustCanonicalJSON(t, &want)
			if gotJSON != wantJSON {
				t.Errorf("evaluated policy does not match chef golden\n--- got ----\n%s\n--- want ---\n%s", gotJSON, wantJSON)
			}
		})
	}
}

// TestEnvIsPlumbedThrough proves Options.Env actually reaches the Policyfile:
// the dyn_cookbook_conditional_env fixture adds the "monitoring" cookbook only
// when WITH_MONITORING != "0". The golden (env unset) includes it; flipping the
// env var must drop it.
func TestEnvIsPlumbedThrough(t *testing.T) {
	engine := NewEngine()
	if err := engine.Available(); err != nil {
		t.Skipf("skipping: ruby.wasm runtime unavailable: %v", err)
	}
	dir := filepath.Join("testdata", "dyn_cookbook_conditional_env")
	source, err := os.ReadFile(filepath.Join(dir, "Policyfile.rb"))
	if err != nil {
		t.Fatal(err)
	}

	withMon, err := engine.Evaluate(context.Background(), string(source), Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := withMon.Cookbooks["monitoring"]; !ok {
		t.Fatalf("expected monitoring cookbook with default env, got %v", keys(withMon.Cookbooks))
	}

	withoutMon, err := engine.Evaluate(context.Background(), string(source), Options{
		Dir: dir,
		Env: map[string]string{"WITH_MONITORING": "0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := withoutMon.Cookbooks["monitoring"]; ok {
		t.Fatalf("expected monitoring cookbook to be dropped when WITH_MONITORING=0, got %v", keys(withoutMon.Cookbooks))
	}
	// And WITH_LOGGING=yes adds logging, proving a second env var flows too.
	withLog, err := engine.Evaluate(context.Background(), string(source), Options{
		Dir: dir,
		Env: map[string]string{"WITH_LOGGING": "yes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := withLog.Cookbooks["logging"]; !ok {
		t.Fatalf("expected logging cookbook with WITH_LOGGING=yes, got %v", keys(withLog.Cookbooks))
	}
}

func mustCanonicalJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func containsAny(errs []string, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
