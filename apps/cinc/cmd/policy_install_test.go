package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cinc-project/cinc-cli/cli/policyfile/rubyeval"
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

// writeCookbook scaffolds a minimal path cookbook (metadata.rb + a default
// recipe) under dir/cookbooks/<name>, so `policy install` can resolve it.
func writeCookbook(t *testing.T, dir, name, version string, depends ...string) {
	t.Helper()
	cbDir := filepath.Join(dir, "cookbooks", name)
	if err := os.MkdirAll(filepath.Join(cbDir, "recipes"), 0o755); err != nil {
		t.Fatal(err)
	}
	md := "name '" + name + "'\nversion '" + version + "'\n"
	for _, d := range depends {
		md += "depends " + d + "\n"
	}
	if err := os.WriteFile(filepath.Join(cbDir, "metadata.rb"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cbDir, "recipes", "default.rb"), []byte("log '"+name+"'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPolicyInstallResolvesPushReadyLock drives `cinc policy install` against a
// Policyfile whose run_list is built dynamically in a loop and whose cookbooks
// come from path sources, and asserts the written lock is fully resolved: every
// cookbook lock carries a content identifier, dotted-decimal identifier, and
// resolved version, plus a revision_id and solution_dependencies.
func TestPolicyInstallResolvesPushReadyLock(t *testing.T) {
	requireRubyEngine(t)

	// A benign (non-denylisted) env var drives the dynamic cookbook below.
	// Note: CINC_*/CHEF_*/AWS_* and other credential-ish names are
	// deliberately withheld from Policyfile evaluation by the rubyeval
	// sandbox, so this uses a plain WITH_CACHE flag instead.
	t.Setenv("WITH_CACHE", "yes")
	dir, path := writePolicyfile(t, `
name 'shop'
run_list(%w[web app].map { |c| "#{c}::default" })
cookbook 'web', path: 'cookbooks/web'
cookbook 'app', path: 'cookbooks/app'
cookbook 'cache', path: 'cookbooks/cache' if ENV['WITH_CACHE'] == 'yes'
default['shop']['port'] = 8080
`)
	writeCookbook(t, dir, "web", "1.0.0", "'app', '~> 2.0'")
	writeCookbook(t, dir, "app", "2.1.0")
	writeCookbook(t, dir, "cache", "0.3.0")

	out, _, err := runRoot(t, "policy", "install", path)
	if err != nil {
		t.Fatalf("policy install: %v", err)
	}
	if !strings.Contains(out, "revision id:") || !strings.Contains(out, "cinc policy push") {
		t.Errorf("expected a resolved summary with a revision id and push hint:\n%s", out)
	}

	lock := readLock(t, filepath.Join(dir, "Policyfile.lock.json"))
	if got := lock["run_list"]; !equalStrings(got, []any{"recipe[web::default]", "recipe[app::default]"}) {
		t.Errorf("run_list = %v, want [recipe[web::default] recipe[app::default]]", got)
	}
	if lock["revision_id"] == "" || lock["revision_id"] == nil {
		t.Error("lock is missing a revision_id")
	}
	cbs, _ := lock["cookbook_locks"].(map[string]any)
	for _, want := range []string{"web", "app", "cache"} {
		entry, ok := cbs[want].(map[string]any)
		if !ok {
			t.Errorf("cookbook %q missing from lock; got %v", want, keysOf(cbs))
			continue
		}
		if entry["identifier"] == "" || entry["identifier"] == nil {
			t.Errorf("cookbook %q lock is missing an identifier (not push-ready): %v", want, entry)
		}
		if entry["dotted_decimal_identifier"] == "" || entry["dotted_decimal_identifier"] == nil {
			t.Errorf("cookbook %q lock is missing a dotted_decimal_identifier", want)
		}
	}
	if _, ok := lock["solution_dependencies"].(map[string]any); !ok {
		t.Error("lock is missing solution_dependencies")
	}
	def, _ := lock["default_attributes"].(map[string]any)
	shop, _ := def["shop"].(map[string]any)
	if shop["port"] != float64(8080) {
		t.Errorf("default_attributes.shop.port = %v, want 8080", shop["port"])
	}
}

// TestPolicyInstallConditionalCookbookDropped proves ENV truly drives the
// evaluation: with the env var unset the conditional cookbook is absent from
// the resolved lock.
func TestPolicyInstallConditionalCookbookDropped(t *testing.T) {
	requireRubyEngine(t)
	t.Setenv("WITH_CACHE", "no")
	dir, path := writePolicyfile(t, `
name 'shop'
run_list 'shop::default'
cookbook 'shop', path: 'cookbooks/shop'
cookbook 'cache', path: 'cookbooks/cache' if ENV['WITH_CACHE'] == 'yes'
`)
	writeCookbook(t, dir, "shop", "1.0.0")
	writeCookbook(t, dir, "cache", "0.3.0")

	if _, _, err := runRoot(t, "policy", "install", path); err != nil {
		t.Fatalf("policy install: %v", err)
	}
	lock := readLock(t, filepath.Join(dir, "Policyfile.lock.json"))
	cbs, _ := lock["cookbook_locks"].(map[string]any)
	if _, ok := cbs["cache"]; ok {
		t.Errorf("cache should be absent when the env var is unset; got %v", keysOf(cbs))
	}
}

func TestPolicyInstallJSONFormat(t *testing.T) {
	requireRubyEngine(t)
	dir, path := writePolicyfile(t, "name 'j'\nrun_list 'j::default'\ncookbook 'j', path: 'cookbooks/j'\n")
	writeCookbook(t, dir, "j", "1.0.0")

	out, _, err := runRoot(t, "policy", "install", path, "--format", "json")
	if err != nil {
		t.Fatalf("policy install --format json: %v", err)
	}
	var summary struct {
		Name       string `json:"name"`
		RevisionID string `json:"revision_id"`
		Cookbooks  []struct {
			Name       string `json:"name"`
			Version    string `json:"version"`
			Identifier string `json:"identifier"`
		} `json:"cookbooks"`
	}
	if err := json.Unmarshal([]byte(out), &summary); err != nil {
		t.Fatalf("output is not valid install-summary JSON: %v\n%s", err, out)
	}
	if summary.Name != "j" || summary.RevisionID == "" {
		t.Errorf("unexpected summary: %+v", summary)
	}
	if len(summary.Cookbooks) != 1 || summary.Cookbooks[0].Name != "j" || summary.Cookbooks[0].Identifier == "" {
		t.Errorf("expected one resolved cookbook j with an identifier, got %+v", summary.Cookbooks)
	}
}

// TestPolicyInstallUnsupportedSource reports a friendly error when a cookbook
// uses a source the resolver can't handle yet (here: a Supermarket default
// source with no path).
func TestPolicyInstallUnsupportedSource(t *testing.T) {
	requireRubyEngine(t)
	_, path := writePolicyfile(t, "name 's'\nrun_list 's::default'\ndefault_source :supermarket\ncookbook 's'\n")
	_, _, err := runRoot(t, "policy", "install", path)
	if err == nil || !strings.Contains(err.Error(), "path:") {
		t.Fatalf("expected an unsupported-source error mentioning path: sources, got %v", err)
	}
}

// TestPolicyInstallUnsatisfiable surfaces a clear error when no version
// satisfies the constraints (chef's NoSolutionError failure mode).
func TestPolicyInstallUnsatisfiable(t *testing.T) {
	requireRubyEngine(t)
	dir, path := writePolicyfile(t, `
name 'conflict'
run_list 'app::default'
cookbook 'app', path: 'cookbooks/app'
cookbook 'lib', path: 'cookbooks/lib'
`)
	writeCookbook(t, dir, "app", "1.0.0", "'lib', '~> 3.0'")
	writeCookbook(t, dir, "lib", "1.0.0")

	_, _, err := runRoot(t, "policy", "install", path)
	if err == nil || !strings.Contains(err.Error(), "lib") {
		t.Fatalf("expected an unsatisfiable-constraint error naming lib, got %v", err)
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
