//go:build acceptance

package acceptance

import (
	"encoding/json"
	"strings"
	"testing"
)

// readCommand is one read-only invocation and the substring its output
// must contain in both human and JSON form.
type readCommand struct {
	name string
	args []string // the cinc args before --config/--format
	want string   // a substring expected in both outputs
}

var readMatrix = []readCommand{
	{"node list", []string{"node", "list"}, "web01"},
	{"node show", []string{"node", "show", "web01"}, "web01"},
	{"role list", []string{"role", "list"}, "base"},
	{"role show", []string{"role", "show", "web"}, "web"},
	{"environment list", []string{"environment", "list"}, "prod"},
	{"environment show", []string{"environment", "show", "prod"}, "prod"},
	{"client list", []string{"client", "list"}, "worker-01"},
	{"client show", []string{"client", "show", "worker-01"}, "worker-01"},
	{"databag list", []string{"databag", "list"}, "users"},
	{"databag item list", []string{"databag", "item", "list", "users"}, "alice"},
	{"group list", []string{"group", "list"}, "admins"},
	{"group show", []string{"group", "show", "admins"}, "admins"},
	{"policy list", []string{"policy", "list"}, "appserver"},
	{"policy show", []string{"policy", "show", "appserver"}, "1.0.0"},
	{"policy-group list", []string{"policy-group", "list"}, "prod"},
	{"policy-group show", []string{"policy-group", "show", "prod"}, "appserver"},
	{"user list", []string{"user", "list"}, "anna"},
}

// TestReadCommandFormatMatrixAgainstCincZero asserts every read command
// produces its expected content in the human default AND emits valid
// JSON under --format json that contains the same substring.
func TestReadCommandFormatMatrixAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	for _, rc := range readMatrix {
		t.Run(rc.name, func(t *testing.T) {
			humanArgs := append(append([]string{}, rc.args...), "--config", env.cfgPath)
			human := runCinc(t, env.binary, humanArgs...)
			if !strings.Contains(human, rc.want) {
				t.Errorf("%s (human) missing %q\ngot: %s", rc.name, rc.want, human)
			}

			jsonArgs := append(append([]string{}, rc.args...), "--config", env.cfgPath, "--format", "json")
			jsonOut := runCinc(t, env.binary, jsonArgs...)
			if !json.Valid([]byte(jsonOut)) {
				t.Errorf("%s (json) is not valid JSON:\n%s", rc.name, jsonOut)
			}
			if !strings.Contains(jsonOut, rc.want) {
				t.Errorf("%s (json) missing %q\ngot: %s", rc.name, rc.want, jsonOut)
			}
		})
	}
}

// notFoundCommand is a show of a name that does not exist.
type notFoundCommand struct {
	name string
	args []string
}

var notFoundMatrix = []notFoundCommand{
	{"node show missing", []string{"node", "show", "ghost"}},
	{"role show missing", []string{"role", "show", "ghost"}},
	{"environment show missing", []string{"environment", "show", "ghost"}},
	{"client show missing", []string{"client", "show", "ghost"}},
	{"group show missing", []string{"group", "show", "ghost"}},
}

// TestNotFoundErrorMatrixAgainstCincZero asserts a show of a missing
// object exits non-zero and surfaces a 404/not-found message rather than
// printing an empty success.
func TestNotFoundErrorMatrixAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	for _, nf := range notFoundMatrix {
		t.Run(nf.name, func(t *testing.T) {
			args := append(append([]string{}, nf.args...), "--config", env.cfgPath)
			_, stderr, err := runCincRaw(env.binary, args...)
			if err == nil {
				t.Fatalf("%s unexpectedly succeeded", nf.name)
			}
			if !strings.Contains(stderr, "404") && !strings.Contains(strings.ToLower(stderr), "not found") {
				t.Errorf("%s stderr does not mention 404/not found:\n%s", nf.name, stderr)
			}
		})
	}
}
