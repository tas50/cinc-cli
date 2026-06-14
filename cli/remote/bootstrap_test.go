package remote

import (
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
		"chef_server_url \"https://cinc.example.test/organizations/acme\"",
		"node_name \"web01\"",
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
