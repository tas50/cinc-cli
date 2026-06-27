//go:build acceptance

package acceptance

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mentionsForbidden reports whether stderr surfaces a 403/forbidden style
// error (cinc-zero phrases these as "missing <verb> permission").
func mentionsForbidden(stderr string) bool {
	low := strings.ToLower(stderr)
	return strings.Contains(stderr, "403") ||
		strings.Contains(low, "forbidden") ||
		strings.Contains(low, "missing") && strings.Contains(low, "permission")
}

// registerRobotClient creates a non-admin "robot" client (signing with a fresh
// keypair) under the admin profile and returns a config that signs as robot.
// It mirrors the setup in auth_test.go's create-path 403 test so the mutation
// 403 cases below stay self-contained.
func registerRobotClient(t *testing.T, env acceptanceEnv) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(t.TempDir(), "robot.pub")
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}), 0o644); err != nil {
		t.Fatal(err)
	}
	runCinc(t, env.binary, "client", "create", "robot", "--public-key", pubPath, "--config", env.cfgPath)

	robotKey := filepath.Join(t.TempDir(), "robot.pem")
	if err := os.WriteFile(robotKey, pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	return writeConfigWithKey(t, env.port, "robot", robotKey)
}

// TestEnforceACLForbidsMutationsAgainstCincZero proves that permission errors
// on *mutation verbs* (delete/edit), not just create, surface as friendly
// 403s with a non-zero exit. The existing auth_test.go only covers the create
// path; this extends that to delete and edit against seeded objects a
// non-admin client may read but not change.
func TestEnforceACLForbidsMutationsAgainstCincZero(t *testing.T) {
	env, stop := startAcceptanceWith(t, acceptanceOptions{enforceACLs: true})
	defer stop()

	robotCfg := registerRobotClient(t, env)

	// A JSON body for the edit cases; the request is refused by authz before
	// the body matters.
	file := filepath.Join(t.TempDir(), "obj.json")
	if err := os.WriteFile(file, []byte(`{"description":"nope"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		args []string
	}{
		{"environment delete", []string{"environment", "delete", "prod"}},
		{"environment edit", []string{"environment", "edit", "prod", "--file", file}},
		{"role delete", []string{"role", "delete", "web"}},
		{"databag delete", []string{"databag", "delete", "users"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, tc.args...), "--config", robotCfg)
			out, stderr, err := runCincRaw(env.binary, args...)
			if err == nil {
				t.Fatalf("non-admin %s unexpectedly succeeded under --enforce-acls\nstdout: %s", tc.name, out)
			}
			if !mentionsForbidden(stderr) {
				t.Errorf("%s stderr does not mention a 403/forbidden/permission error:\n%s", tc.name, stderr)
			}
		})
	}
}
