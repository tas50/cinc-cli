//go:build acceptance

package acceptance

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

// ACLs are exposed per-noun as an `acl` subgroup (e.g. `cinc node acl show`).
// The acceptance harness signs as the pivotal superuser, which holds grant on
// every object, so the read-modify-write flow is exercisable here against the
// seeded "acme" org.
//
// Where the pinned cinc-zero build doesn't implement an _acl endpoint, the
// test skips with a documented reason and the behavior stays covered by the
// unit tests in apps/cinc/cmd/acl_test.go (which assert the exact GET-then-PUT
// wiring, the merged ACE body, `grant all`, the no-op path, and the org/user
// special endpoints).

// TestNodeACLShowAgainstCincZero fetches the ACL of a seeded node and asserts
// it parses into the five-permission shape.
func TestNodeACLShowAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out, stderr, err := runCincRaw(env.binary, "node", "acl", "show", "web01",
		"--config", env.cfgPath, "--format", "json")
	if err != nil {
		t.Skipf("cinc-zero does not serve node ACLs; covered by unit tests. stderr: %s", stderr)
	}
	var acl cinc.ACL
	if err := json.Unmarshal([]byte(out), &acl); err != nil {
		t.Fatalf("node acl show (json) not valid JSON: %v\n%s", err, out)
	}
	// A real ACL always grants read to at least one group or actor.
	if len(acl.Read.Groups) == 0 && len(acl.Read.Actors) == 0 {
		t.Errorf("node web01 read ACE is empty, which is implausible for a seeded node:\n%s", out)
	}
}

// TestNodeACLGrantRevokeRoundTripAgainstCincZero grants the seeded "devs"
// group read on a node, confirms it via show, then revokes it and confirms it
// is gone. Skips if the pinned cinc-zero rejects the ACL GET or PUT.
func TestNodeACLGrantRevokeRoundTripAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	grant, stderr, err := runCincRaw(env.binary, "node", "acl", "grant", "read", "web01",
		"--group", "devs", "--config", env.cfgPath)
	if err != nil {
		t.Skipf("cinc-zero does not support setting node ACLs; covered by unit tests. stderr: %s", stderr)
	}
	if !strings.Contains(grant, "devs") {
		t.Errorf("node acl grant output should mention devs:\n%s", grant)
	}

	afterGrant := readNodeACL(t, env, "web01")
	if !slices.Contains(afterGrant.Read.Groups, "devs") {
		t.Errorf("after grant, read groups = %v, want it to contain devs", afterGrant.Read.Groups)
	}

	revoke := runCinc(t, env.binary, "node", "acl", "revoke", "read", "web01",
		"--group", "devs", "--config", env.cfgPath)
	if !strings.Contains(revoke, "devs") {
		t.Errorf("node acl revoke output should mention devs:\n%s", revoke)
	}

	afterRevoke := readNodeACL(t, env, "web01")
	if slices.Contains(afterRevoke.Read.Groups, "devs") {
		t.Errorf("after revoke, read groups = %v, want devs removed", afterRevoke.Read.Groups)
	}
}

// TestNodeACLGrantNoOpAgainstCincZero re-grants an existing member and expects
// the friendly no-op message rather than an error.
func TestNodeACLGrantNoOpAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	acl := readNodeACLOrSkip(t, env, "web01")
	if len(acl.Read.Groups) == 0 {
		t.Skip("seeded node has no read groups to re-grant; covered by unit tests")
	}
	existing := acl.Read.Groups[0]

	out := runCinc(t, env.binary, "node", "acl", "grant", "read", "web01",
		"--group", existing, "--config", env.cfgPath)
	if !strings.Contains(strings.ToLower(out), "no change") {
		t.Errorf("re-granting %q should be a friendly no-op:\n%s", existing, out)
	}
}

// TestOrgACLShowAgainstCincZero fetches the organization's own ACL (the
// nameless /organizations/acme/_acl endpoint).
func TestOrgACLShowAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out, stderr, err := runCincRaw(env.binary, "org", "acl", "show",
		"--config", env.cfgPath, "--format", "json")
	if err != nil {
		t.Skipf("cinc-zero does not serve org ACLs; covered by unit tests. stderr: %s", stderr)
	}
	var acl cinc.ACL
	if err := json.Unmarshal([]byte(out), &acl); err != nil {
		t.Fatalf("org acl show (json) not valid JSON: %v\n%s", err, out)
	}
}

func readNodeACL(t *testing.T, env acceptanceEnv, name string) cinc.ACL {
	t.Helper()
	out := runCinc(t, env.binary, "node", "acl", "show", name, "--config", env.cfgPath, "--format", "json")
	var acl cinc.ACL
	if err := json.Unmarshal([]byte(out), &acl); err != nil {
		t.Fatalf("node acl show (json) not valid JSON: %v\n%s", err, out)
	}
	return acl
}

func readNodeACLOrSkip(t *testing.T, env acceptanceEnv, name string) cinc.ACL {
	t.Helper()
	out, stderr, err := runCincRaw(env.binary, "node", "acl", "show", name, "--config", env.cfgPath, "--format", "json")
	if err != nil {
		t.Skipf("cinc-zero does not serve node ACLs; covered by unit tests. stderr: %s", stderr)
	}
	var acl cinc.ACL
	if err := json.Unmarshal([]byte(out), &acl); err != nil {
		t.Fatalf("node acl show (json) not valid JSON: %v\n%s", err, out)
	}
	return acl
}
