//go:build acceptance

package acceptance

import (
	"strings"
	"testing"
)

// TestClientKeyLifecycleAgainstCincZero adds a second key to a seeded
// client, confirms it via list/show, then deletes it. The server
// generates the key pair and returns the private key, which the keys
// API surfaces at the top level of the response (unlike `client
// create`, whose key the CLI cannot reach against cinc-zero) — so the
// server-generated path is exercised here directly.
func TestClientKeyLifecycleAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	create := runCinc(t, env.binary, "client", "key", "create", "worker-01", "rotation", "--config", env.cfgPath)
	if !strings.Contains(create, "BEGIN RSA PRIVATE KEY") {
		t.Errorf("client key create did not stream a private key:\n%s", create)
	}

	list := runCinc(t, env.binary, "client", "key", "list", "worker-01", "--config", env.cfgPath)
	if !strings.Contains(list, "rotation") {
		t.Errorf("client key list missing the new key:\n%s", list)
	}

	show := runCinc(t, env.binary, "client", "key", "show", "worker-01", "rotation", "--config", env.cfgPath)
	if !strings.Contains(show, "\"name\": \"rotation\"") {
		t.Errorf("client key show missing name field:\n%s", show)
	}

	del := runCinc(t, env.binary, "client", "key", "delete", "worker-01", "rotation", "--config", env.cfgPath)
	if del != "Deleted key \"rotation\" from client \"worker-01\"\n" {
		t.Errorf("client key delete output = %q", del)
	}

	after := runCinc(t, env.binary, "client", "key", "list", "worker-01", "--config", env.cfgPath)
	if strings.Contains(after, "rotation") {
		t.Errorf("client key list still has the deleted key:\n%s", after)
	}
}

// TestUserKeyLifecycleAgainstCincZero exercises the same flow on the
// per-user (global, non-org-scoped) keys path, proving the shared key
// builder targets the right collection when wired under `cinc user`.
func TestUserKeyLifecycleAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	create := runCinc(t, env.binary, "user", "key", "create", "anna", "laptop", "--config", env.cfgPath)
	if !strings.Contains(create, "BEGIN RSA PRIVATE KEY") {
		t.Errorf("user key create did not stream a private key:\n%s", create)
	}

	list := runCinc(t, env.binary, "user", "key", "list", "anna", "--config", env.cfgPath)
	if !strings.Contains(list, "laptop") {
		t.Errorf("user key list missing the new key:\n%s", list)
	}

	del := runCinc(t, env.binary, "user", "key", "delete", "anna", "laptop", "--config", env.cfgPath)
	if del != "Deleted key \"laptop\" from user \"anna\"\n" {
		t.Errorf("user key delete output = %q", del)
	}
}

// TestClientReregisterAgainstCincZero creates a fresh client (so a
// default key exists to regenerate), reregisters it, and confirms a new
// private key is streamed back.
func TestClientReregisterAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	// `client create` without --public-key has cinc-zero generate the
	// pair, leaving a default key on the server for reregister to
	// regenerate. runCinc fails the test if the command errors; the
	// streamed key itself is not asserted on here.
	runCinc(t, env.binary, "client", "create", "rereg01", "--config", env.cfgPath)

	out := runCinc(t, env.binary, "client", "reregister", "rereg01", "--config", env.cfgPath)
	if !strings.Contains(out, "BEGIN RSA PRIVATE KEY") {
		t.Errorf("client reregister did not stream a new private key:\n%s", out)
	}

	// The client still has exactly a default key after reregister.
	list := runCinc(t, env.binary, "client", "key", "list", "rereg01", "--config", env.cfgPath)
	if !strings.Contains(list, "default") {
		t.Errorf("client key list after reregister missing default key:\n%s", list)
	}
}
