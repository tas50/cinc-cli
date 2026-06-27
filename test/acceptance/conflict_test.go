//go:build acceptance

package acceptance

import (
	"strings"
	"testing"
)

// mentionsConflict reports whether stderr surfaces a 409/already-exists style
// error, however the server phrases it ("Object already exists", "The
// association already exists.", etc.).
func mentionsConflict(stderr string) bool {
	low := strings.ToLower(stderr)
	return strings.Contains(stderr, "409") ||
		strings.Contains(low, "conflict") ||
		strings.Contains(low, "already exist")
}

// conflictCase creates an object that already exists and expects the server to
// refuse it with a conflict. setup runs first (each command must succeed,
// unless optional) so the object is present; again is the create expected to
// conflict.
type conflictCase struct {
	name  string
	setup [][]string // commands run before `again`; args before --config
	again []string   // the create expected to conflict; args before --config
	// optional marks a case whose endpoint the pinned cinc-zero may not
	// implement (org membership/invitations). If setup fails or the repeat
	// create unexpectedly succeeds, the subtest skips rather than fails — the
	// behavior stays covered by the unit tests in apps/cinc/cmd.
	optional bool
}

// conflictMatrix re-creates an object that the seed (or an explicit setup step)
// already established and asserts the server reports a conflict. The seeded
// objects come from test/acceptance/seed and the harness's seedGlobalActors
// (the global user "anna").
var conflictMatrix = []conflictCase{
	{name: "node", again: []string{"node", "create", "web01", "--run-list", "recipe[base]"}},
	{name: "role", again: []string{"role", "create", "web", "--description", "dup"}},
	{name: "environment", again: []string{"environment", "create", "prod", "--description", "dup"}},
	{name: "client", again: []string{"client", "create", "worker-01"}},
	{name: "group", again: []string{"group", "create", "admins"}},
	{name: "databag", again: []string{"databag", "create", "users"}},
	{name: "user", again: []string{"user", "create", "anna",
		"--email", "anna@example.test", "--display-name", "Anna Admin",
		"--first-name", "Anna", "--last-name", "Admin"}},
	{
		name:     "org member add",
		setup:    [][]string{{"org", "member", "add", "anna"}},
		again:    []string{"org", "member", "add", "anna"},
		optional: true,
	},
	{
		name:     "org invite create",
		setup:    [][]string{{"org", "invite", "create", "ben"}},
		again:    []string{"org", "invite", "create", "ben"},
		optional: true,
	},
	{
		name:  "client key create",
		setup: [][]string{{"client", "key", "create", "worker-01", "rotation"}},
		again: []string{"client", "key", "create", "worker-01", "rotation"},
	},
}

// TestCreateConflictMatrixAgainstCincZero asserts that re-creating an object
// that already exists exits non-zero and surfaces a friendly conflict message
// rather than silently overwriting it. This is the 409 complement to the
// happy-path create tests, which only ever create fresh names.
func TestCreateConflictMatrixAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	for _, tc := range conflictMatrix {
		t.Run(tc.name, func(t *testing.T) {
			for _, s := range tc.setup {
				args := append(append([]string{}, s...), "--config", env.cfgPath)
				if _, stderr, err := runCincRaw(env.binary, args...); err != nil {
					if tc.optional {
						t.Skipf("cinc-zero does not support %q; covered by unit tests. stderr: %s",
							strings.Join(s, " "), strings.TrimSpace(stderr))
					}
					t.Fatalf("setup %q failed: %s", strings.Join(s, " "), stderr)
				}
			}

			args := append(append([]string{}, tc.again...), "--config", env.cfgPath)
			out, stderr, err := runCincRaw(env.binary, args...)
			if err == nil {
				if tc.optional {
					t.Skipf("cinc-zero allowed a duplicate %s; covered by unit tests.\nstdout: %s", tc.name, out)
				}
				t.Fatalf("duplicate %s create unexpectedly succeeded\nstdout: %s", tc.name, out)
			}
			if !mentionsConflict(stderr) {
				t.Errorf("%s stderr does not mention a conflict/already-exists/409:\n%s", tc.name, stderr)
			}
		})
	}
}
