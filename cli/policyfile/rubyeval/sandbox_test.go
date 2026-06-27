package rubyeval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSandboxConstants documents the host-side caps applied to every guest
// evaluation (findings 1 & 2): a bounded default timeout, a guest memory page
// cap (512 MiB), and a result-size cap (64 MiB).
func TestSandboxConstants(t *testing.T) {
	if defaultEvalTimeout <= 0 {
		t.Errorf("defaultEvalTimeout must be positive, got %v", defaultEvalTimeout)
	}
	if maxMemoryPages != 8192 {
		t.Errorf("maxMemoryPages should cap guest memory at 512 MiB (8192 pages), got %d", maxMemoryPages)
	}
	if maxResultBytes != 64<<20 {
		t.Errorf("maxResultBytes should be 64 MiB, got %d", maxResultBytes)
	}
}

// TestReadCappedRejectsOversizeResult proves the result reader refuses an
// out.json larger than the cap instead of buffering it (finding 2). It writes
// an oversize file directly and asserts the over-cap error.
func TestReadCappedRejectsOversizeResult(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "out.json")
	if err := os.WriteFile(big, make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCapped(big, 1024); err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("expected an over-cap error, got %v", err)
	}

	// An under-cap file reads back verbatim.
	small := filepath.Join(dir, "small.json")
	if err := os.WriteFile(small, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readCapped(small, 1024)
	if err != nil {
		t.Fatalf("under-cap read should succeed: %v", err)
	}
	if string(got) != "{}" {
		t.Errorf("got %q, want %q", got, "{}")
	}
}

// TestCopySiblingsSkipsSymlinks proves a symlinked sibling pointing outside the
// policy dir is NOT copied into the sandbox (finding 4), so the guest cannot
// File.read a host secret it links to.
func TestCopySiblingsSkipsSymlinks(t *testing.T) {
	// A secret living outside the policy directory.
	outside := filepath.Join(t.TempDir(), "host_secret.txt")
	if err := os.WriteFile(outside, []byte("TOP SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}

	src := t.TempDir()
	// A legitimate regular sibling — must still be copied.
	if err := os.WriteFile(filepath.Join(src, "helper.rb"), []byte("# ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A malicious symlink pointing at the outside secret.
	if err := os.Symlink(outside, filepath.Join(src, "evil.rb")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	dst := t.TempDir()
	if err := copySiblings(src, dst, "Policyfile.rb"); err != nil {
		t.Fatalf("copySiblings: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(dst, "evil.rb")); !os.IsNotExist(err) {
		t.Errorf("symlinked sibling must not be copied into the sandbox (lstat err: %v)", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "helper.rb")); err != nil {
		t.Errorf("regular sibling should still be copied: %v", err)
	}
}

// TestIsSensitiveEnvName documents the Policyfile env denylist (finding 3).
func TestIsSensitiveEnvName(t *testing.T) {
	deny := []string{
		"AWS_SECRET_ACCESS_KEY", "aws_session_token", "CINC_SERVER_URL",
		"CHEF_PROFILE", "MY_API_TOKEN", "db_password", "TLS_PRIVATE_KEY",
		"GITHUB_KEY", "some_secret_thing",
	}
	for _, n := range deny {
		if !isSensitiveEnvName(n) {
			t.Errorf("expected %q to be treated as sensitive", n)
		}
	}
	keep := []string{"HOME", "PATH", "LANG", "WITH_MONITORING", "PF_BENIGN_VAR"}
	for _, n := range keep {
		if isSensitiveEnvName(n) {
			t.Errorf("expected %q to be kept (not sensitive)", n)
		}
	}
}

// TestEvaluationTimesOut proves a runaway guest is interrupted rather than
// hanging the host (finding 1). It warms the compile cache with a quick
// evaluation, then runs an infinite loop under a short Options.Timeout and
// asserts a prompt timeout error.
func TestEvaluationTimesOut(t *testing.T) {
	engine := NewEngine()
	if err := engine.Available(); err != nil {
		t.Skipf("skipping: ruby.wasm runtime unavailable: %v", err)
	}

	// Warm the (slow, cached) wasm compile so the short timeout below bites the
	// guest loop, not the one-time compilation.
	warm := "name \"warm\"\nrun_list \"recipe[app::default]\"\n"
	if _, err := engine.Evaluate(context.Background(), warm, Options{}); err != nil {
		t.Fatalf("warm-up evaluation failed: %v", err)
	}

	start := time.Now()
	_, err := engine.Evaluate(context.Background(), "loop {}", Options{Timeout: 3 * time.Second})
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected a timeout error, got %v", err)
	}
	if elapsed > 30*time.Second {
		t.Fatalf("timeout did not interrupt the guest promptly: took %v", elapsed)
	}
}

// TestSensitiveEnvNotLeakedToGuest proves that a sensitive host env var set in
// the test process is NOT visible to either an attacker-controllable
// Policyfile.rb (denylist-filtered env) or a cookbook metadata.rb (empty env),
// while a benign var still reaches the Policyfile for chef-compat (finding 3).
func TestSensitiveEnvNotLeakedToGuest(t *testing.T) {
	engine := NewEngine()
	if err := engine.Available(); err != nil {
		t.Skipf("skipping: ruby.wasm runtime unavailable: %v", err)
	}

	t.Setenv("AWS_SECRET_ACCESS_KEY", "super-secret-value")
	t.Setenv("PF_BENIGN_VAR", "benign-value")

	// Policyfile path: secret stripped, benign kept.
	pfDir := t.TempDir()
	pf := filepath.Join(pfDir, "Policyfile.rb")
	pfSrc := `name "envtest"
default["secret"] = ENV["AWS_SECRET_ACCESS_KEY"] || "absent"
default["benign"] = ENV["PF_BENIGN_VAR"] || "absent"
run_list "recipe[app::default]"
`
	if err := os.WriteFile(pf, []byte(pfSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	pol, err := engine.EvaluateFile(context.Background(), pf)
	if err != nil {
		t.Fatalf("EvaluateFile: %v", err)
	}
	if got := pol.DefaultAttributes["secret"]; got != "absent" {
		t.Errorf("Policyfile leaked AWS_SECRET_ACCESS_KEY: got %v, want \"absent\"", got)
	}
	if got := pol.DefaultAttributes["benign"]; got != "benign-value" {
		t.Errorf("Policyfile should still see benign env: got %v, want \"benign-value\"", got)
	}

	// metadata.rb path: NO env at all, so the secret is absent.
	mdDir := t.TempDir()
	md := filepath.Join(mdDir, "metadata.rb")
	mdSrc := `name (ENV["AWS_SECRET_ACCESS_KEY"] || "absent")
version "1.0.0"
`
	if err := os.WriteFile(md, []byte(mdSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	meta, err := engine.EvaluateMetadataFile(context.Background(), md)
	if err != nil {
		t.Fatalf("EvaluateMetadataFile: %v", err)
	}
	if meta.Name != "absent" {
		t.Errorf("metadata.rb leaked AWS_SECRET_ACCESS_KEY: got %q, want \"absent\"", meta.Name)
	}
}
