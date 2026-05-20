//go:build acceptance

package acceptance

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestSetupMigratesChefCredentialsOnFirstRun seeds ~/.chef/credentials
// in a tempdir HOME, runs `cinc node list` with a pseudo-tty on stdin
// so the migration prompt actually fires, answers "y", and expects a
// successful node list.
func TestSetupMigratesChefCredentialsOnFirstRun(t *testing.T) {
	requireChefZero(t)
	port := freePort(t)
	stop := startChefZero(t, port)
	defer stop()
	binary := buildCinc(t)

	home := t.TempDir()
	keyPath := filepath.Join(home, ".chef", "tester.pem")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, generateKeyPEM(t), 0o600); err != nil {
		t.Fatal(err)
	}
	chefCreds := fmt.Sprintf(`[default]
chef_server_url = "http://127.0.0.1:%d/organizations/acme"
client_name     = "tester"
client_key      = %q
`, port, keyPath)
	if err := os.WriteFile(filepath.Join(home, ".chef", "credentials"), []byte(chefCreds), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, "node", "list")
	cmd.Env = append(os.Environ(), "HOME="+home)

	stdout, stderr, err := runWithTTYStdin(t, cmd, "y\n")
	if err != nil {
		t.Fatalf("cinc node list after migration prompt: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "Welcome to cinc!") {
		t.Errorf("expected welcome on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "migrate it") {
		t.Errorf("expected migration prompt on stderr, got:\n%s", stderr)
	}
	cincFile := filepath.Join(home, ".cinc", "credentials")
	cincContents, err := os.ReadFile(cincFile)
	if err != nil {
		t.Fatalf("expected %s to exist after migration: %v", cincFile, err)
	}
	if !strings.Contains(string(cincContents), "chef_server_url") {
		t.Errorf("migrated credentials should contain a server URL, got:\n%s", cincContents)
	}
}

// runWithTTYStdin starts cmd with a pseudo-tty slave as stdin so the
// child's TTY guard recognises an interactive session, while stdout
// and stderr stay on ordinary buffers we can read directly. answer is
// written to the master after a brief delay so the child has time to
// print its prompt and block on read.
func runWithTTYStdin(t *testing.T, cmd *exec.Cmd, answer string) (string, string, error) {
	t.Helper()
	ptmx, tty, err := pty.Open()
	if err != nil {
		return "", "", fmt.Errorf("pty.Open: %w", err)
	}
	defer ptmx.Close()

	var stdout, stderr bytes.Buffer
	cmd.Stdin = tty
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		tty.Close()
		return "", "", fmt.Errorf("cmd.Start: %w", err)
	}
	// The parent no longer needs the slave; the child has its own
	// duplicate.
	tty.Close()

	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(ptmx, answer)
	}()

	waitErr := cmd.Wait()
	return stdout.String(), stderr.String(), waitErr
}

func generateKeyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}
