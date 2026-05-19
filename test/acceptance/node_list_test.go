//go:build acceptance

package acceptance

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// cincPackage is the import path of the cinc binary's main package.
const cincPackage = "github.com/tas50/cinc-cli/apps/cinc"

// TestNodeListAgainstChefZero runs the real cinc binary against a live
// chef-zero server: it seeds three nodes, then asserts that `cinc node
// list` reports them, in both output formats.
func TestNodeListAgainstChefZero(t *testing.T) {
	requireChefZero(t)

	port := freePort(t)
	stop := startChefZero(t, port)
	defer stop()

	binary := buildCinc(t)
	cfgPath := writeAcceptanceConfig(t, port)

	human := runCinc(t, binary, "node", "list", "--config", cfgPath)
	if human != "db01\nweb01\nweb02\n" {
		t.Errorf("node list (human) = %q, want sorted node names", human)
	}

	jsonOut := runCinc(t, binary, "node", "list", "--config", cfgPath, "--format", "json")
	for _, name := range []string{"db01", "web01", "web02"} {
		if !strings.Contains(jsonOut, name) {
			t.Errorf("node list (json) missing %q\ngot: %s", name, jsonOut)
		}
	}
}

// requireChefZero skips the test unless Ruby and the chef-zero gem are
// available.
func requireChefZero(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ruby"); err != nil {
		t.Skip("ruby not found on PATH")
	}
	if err := exec.Command("ruby", "-e", "require 'chef_zero/server'").Run(); err != nil {
		t.Skip("chef-zero gem not installed (run: gem install chef-zero)")
	}
}

// freePort reserves and returns an unused TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startChefZero launches the chef-zero helper on port and blocks until it
// is serving requests. It returns a function that shuts the server down.
func startChefZero(t *testing.T, port int) func() {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the acceptance test source directory")
	}
	script := filepath.Join(filepath.Dir(thisFile), "chef-zero-server.rb")

	cmd := exec.Command("ruby", script, fmt.Sprint(port), "acme")
	var log bytes.Buffer
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting chef-zero: %v", err)
	}
	stop := func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/organizations/acme/nodes", port)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return stop
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	stop()
	t.Fatalf("chef-zero did not become ready on port %d within 30s\noutput:\n%s", port, log.String())
	return nil
}

// buildCinc compiles the cinc binary into a temp directory and returns its
// path.
func buildCinc(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "cinc")
	out, err := exec.Command("go", "build", "-o", binary, cincPackage).CombinedOutput()
	if err != nil {
		t.Fatalf("building cinc: %v\n%s", err, out)
	}
	return binary
}

// writeAcceptanceConfig writes a config file (and a throwaway signing key)
// pointing at the chef-zero server, and returns the config path.
func writeAcceptanceConfig(t *testing.T, port int) string {
	t.Helper()
	dir := t.TempDir()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "key.pem")
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "credentials")
	cfg := fmt.Sprintf(`[default]
cinc_server_url = "http://127.0.0.1:%d/organizations/acme"
client_name     = "tester"
client_key      = %q
`, port, keyPath)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

// runCinc executes the cinc binary, fails the test on a non-zero exit, and
// returns its standard output.
func runCinc(t *testing.T, binary string, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binary, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("cinc %s failed: %v\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String()
}
