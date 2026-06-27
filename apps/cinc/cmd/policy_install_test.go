package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tas50/cinc-cli/cli/policyfile/rubyeval"
)

// requireRubyEngine skips the test cleanly when the pinned ruby.wasm runtime
// cannot be fetched (e.g. offline CI), and otherwise lets the test run the real
// embedded engine.
func requireRubyEngine(t *testing.T) {
	t.Helper()
	if err := rubyeval.NewEngine().Available(); err != nil {
		if rubyeval.IsUnavailable(err) {
			t.Skipf("skipping: embedded ruby.wasm runtime unavailable: %v", err)
		}
		t.Fatalf("unexpected ruby.wasm availability error: %v", err)
	}
}

func writePolicyfile(t *testing.T, body string) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "Policyfile.rb")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

// TestPolicyInstallEvaluatesDynamicPolicyfile drives `cinc policy install`
// against a Policyfile that builds its run_list in a loop and adds a cookbook
// conditionally from an environment variable — proving the command runs the
// real embedded-Ruby engine — then checks the written lock reflects the
// dynamically-produced declarations.
func TestPolicyInstallEvaluatesDynamicPolicyfile(t *testing.T) {
	requireRubyEngine(t)

	t.Setenv("CINC_TEST_WITH_CACHE", "yes")
	dir, path := writePolicyfile(t, `
name 'shop'
default_source :supermarket
run_list(%w[web app].map { |t| "role[#{t}]" })
cookbook 'shop', path: '.'
cookbook 'redis', '~> 5.0'
cookbook 'memcached' if ENV['CINC_TEST_WITH_CACHE'] == 'yes'
default['shop']['port'] = 8080
`)

	out, _, err := runRoot(t, "policy", "install", path)
	if err != nil {
		t.Fatalf("policy install: %v", err)
	}
	if !strings.Contains(out, "evaluation-only lock") {
		t.Errorf("expected the evaluation-only boundary note in output:\n%s", out)
	}

	lock := readLock(t, filepath.Join(dir, "Policyfile.lock.json"))
	if got := lock["run_list"]; !equalStrings(got, []any{"role[web]", "role[app]"}) {
		t.Errorf("run_list = %v, want [role[web] role[app]] from the loop", got)
	}
	cbs, _ := lock["cookbook_locks"].(map[string]any)
	for _, want := range []string{"shop", "redis", "memcached"} {
		if _, ok := cbs[want]; !ok {
			t.Errorf("cookbook %q missing from lock; got %v", want, keysOf(cbs))
		}
	}
	def, _ := lock["default_attributes"].(map[string]any)
	shop, _ := def["shop"].(map[string]any)
	if shop["port"] != float64(8080) {
		t.Errorf("default_attributes.shop.port = %v, want 8080", shop["port"])
	}
}

// TestPolicyInstallConditionalCookbookDropped proves ENV truly drives the
// evaluation: with the env var unset the conditional cookbook is absent.
func TestPolicyInstallConditionalCookbookDropped(t *testing.T) {
	requireRubyEngine(t)
	t.Setenv("CINC_TEST_WITH_CACHE", "no")
	dir, path := writePolicyfile(t, `
name 'shop'
run_list 'recipe[shop::default]'
cookbook 'memcached' if ENV['CINC_TEST_WITH_CACHE'] == 'yes'
`)
	if _, _, err := runRoot(t, "policy", "install", path); err != nil {
		t.Fatalf("policy install: %v", err)
	}
	lock := readLock(t, filepath.Join(dir, "Policyfile.lock.json"))
	cbs, _ := lock["cookbook_locks"].(map[string]any)
	if _, ok := cbs["memcached"]; ok {
		t.Errorf("memcached should be absent when the env var is unset; got %v", keysOf(cbs))
	}
}

func TestPolicyInstallJSONFormat(t *testing.T) {
	requireRubyEngine(t)
	_, path := writePolicyfile(t, "name 'j'\nrun_list 'recipe[j::default]'\ncookbook 'j', '= 1.0.0'\n")
	out, _, err := runRoot(t, "policy", "install", path, "--format", "json")
	if err != nil {
		t.Fatalf("policy install --format json: %v", err)
	}
	var eval rubyeval.EvaluatedPolicy
	if err := json.Unmarshal([]byte(out), &eval); err != nil {
		t.Fatalf("output is not valid EvaluatedPolicy JSON: %v\n%s", err, out)
	}
	if eval.Name != "j" || eval.Cookbooks["j"].VersionConstraint != "= 1.0.0" {
		t.Errorf("unexpected evaluation: %+v", eval)
	}
}

func TestPolicyInstallSurfacesSyntaxError(t *testing.T) {
	requireRubyEngine(t)
	_, path := writePolicyfile(t, "name 'broken'\nrun_list 'recipe[x]'\ncookbook 'x',\n")
	_, _, err := runRoot(t, "policy", "install", path)
	if err == nil || !strings.Contains(err.Error(), "syntax error") {
		t.Fatalf("expected a surfaced syntax error, got %v", err)
	}
}

func TestPolicyInstallSurfacesRaise(t *testing.T) {
	requireRubyEngine(t)
	_, path := writePolicyfile(t, "name 'boom'\nrun_list 'recipe[x]'\nraise 'kaboom'\n")
	_, _, err := runRoot(t, "policy", "install", path)
	if err == nil || !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("expected the raised message to be surfaced, got %v", err)
	}
}

func TestPolicyInstallMissingFile(t *testing.T) {
	// No engine needed: the missing-file check happens before evaluation.
	_, _, err := runRoot(t, "policy", "install", filepath.Join(t.TempDir(), "nope.rb"))
	if err == nil || !strings.Contains(err.Error(), "couldn't find a Policyfile") {
		t.Fatalf("expected a friendly missing-file error, got %v", err)
	}
}

func readLock(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("lock is not valid JSON: %v", err)
	}
	return m
}

func equalStrings(got any, want []any) bool {
	arr, ok := got.([]any)
	if !ok || len(arr) != len(want) {
		return false
	}
	for i := range want {
		if arr[i] != want[i] {
			return false
		}
	}
	return true
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
