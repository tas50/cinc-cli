package remote

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBootstrapCommandBuildsCincClientScript(t *testing.T) {
	cmd, err := BootstrapCommand(BootstrapOptions{
		NodeName:         "web01",
		ServerURL:        "https://cinc.example.test/organizations/acme",
		ClientKeyPEM:     "PRIVATE KEY",
		RunList:          []string{"recipe[apt]"},
		Environment:      "prod",
		Sudo:             true,
		BootstrapURL:     DefaultBootstrapURL,
		BootstrapVersion: "18",
	})
	if err != nil {
		t.Fatalf("BootstrapCommand: %v", err)
	}
	for _, want := range []string{
		"curl -L 'https://omnitruck.cinc.sh/install.sh' | sudo bash -s -- -v '18'",
		"sudo mkdir -p /etc/cinc",
		"sudo install -m 0600 /dev/null /etc/cinc/client.pem",
		"chef_server_url 'https://cinc.example.test/organizations/acme'",
		"node_name 'web01'",
		"PRIVATE KEY",
		"\"chef_environment\": \"prod\"",
		"\"recipe[apt]\"",
		"sudo cinc-client -j /etc/cinc/first-boot.json",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("command missing %q:\n%s", want, cmd)
		}
	}
}

// firstBoot extracts and decodes the first-boot.json payload embedded in the
// generated bootstrap script so tests can assert on the actual JSON keys.
func firstBoot(t *testing.T, opts BootstrapOptions) map[string]any {
	t.Helper()
	data, err := firstBootJSON(opts)
	if err != nil {
		t.Fatalf("firstBootJSON: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode first-boot.json: %v", err)
	}
	return doc
}

// A Policyfile-managed node is mutually exclusive with chef_environment;
// cinc-client/chef-client treats setting both as a conflict. The bootstrap
// --environment flag defaults to "_default", so a policy bootstrap must drop
// chef_environment from first-boot.json entirely.
func TestFirstBootJSONOmitsEnvironmentForPolicyBootstrap(t *testing.T) {
	doc := firstBoot(t, BootstrapOptions{
		NodeName:    "web01",
		ServerURL:   "https://cinc.example.test/organizations/acme",
		PolicyName:  "base",
		PolicyGroup: "prod",
		Environment: "_default", // the flag default that leaks the conflict
	})
	if _, ok := doc["chef_environment"]; ok {
		t.Fatalf("policy bootstrap first-boot.json must not set chef_environment: %v", doc)
	}
	if doc["policy_name"] != "base" || doc["policy_group"] != "prod" {
		t.Fatalf("policy fields missing or wrong: %v", doc)
	}
}

// client.rb is evaluated as Ruby on the target. Ruby double-quoted strings
// interpolate #{...}, so values must be emitted as single-quoted literals to
// avoid arbitrary code execution from a node name / server URL.
func TestClientRBDoesNotInterpolateRuby(t *testing.T) {
	rb := clientRB(BootstrapOptions{
		NodeName:  `web#{exec("id")}01`,
		ServerURL: "https://cinc.example.test/organizations/acme",
	})
	if !strings.Contains(rb, `node_name 'web#{exec("id")}01'`) {
		t.Fatalf("node_name should be a single-quoted literal, got:\n%s", rb)
	}
	if strings.Contains(rb, `node_name "`) {
		t.Fatalf("node_name must not be an interpolating double-quoted string:\n%s", rb)
	}
}

// Single-quoted Ruby literals still need backslash and single-quote escaping.
func TestClientRBEscapesQuotesAndBackslashes(t *testing.T) {
	rb := clientRB(BootstrapOptions{
		NodeName:  `it's\ok`,
		ServerURL: "https://x/organizations/o",
	})
	if !strings.Contains(rb, `node_name 'it\'s\\ok'`) {
		t.Fatalf("node_name escaping wrong, got:\n%s", rb)
	}
}

// A run-list bootstrap (no policy) keeps chef_environment as before.
func TestFirstBootJSONKeepsEnvironmentForRunListBootstrap(t *testing.T) {
	doc := firstBoot(t, BootstrapOptions{
		NodeName:    "web01",
		ServerURL:   "https://cinc.example.test/organizations/acme",
		RunList:     []string{"recipe[apt]"},
		Environment: "prod",
	})
	if doc["chef_environment"] != "prod" {
		t.Fatalf("run-list bootstrap should keep chef_environment: %v", doc)
	}
}
