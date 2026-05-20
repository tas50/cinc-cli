package remote

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
)

const DefaultBootstrapURL = "https://omnitruck.cinc.sh/install.sh"

// BootstrapOptions describes the remote Cinc Client bootstrap script.
type BootstrapOptions struct {
	NodeName         string
	ServerURL        string
	ClientKeyPEM     string
	RunList          []string
	Environment      string
	PolicyName       string
	PolicyGroup      string
	Sudo             bool
	BootstrapURL     string
	BootstrapVersion string
}

// GenerateClientKeyPair returns a PEM private/public RSA key pair for a
// bootstrapped node client. The public key is registered with the server; the
// private key is installed onto the target host.
func GenerateClientKeyPair() (privatePEM, publicPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	privateBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	publicBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", err
	}
	publicPEMBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicBytes,
	})
	return string(privateBytes), string(publicPEMBytes), nil
}

// BootstrapCommand builds the shell command executed on the target.
func BootstrapCommand(opts BootstrapOptions) (string, error) {
	if opts.NodeName == "" {
		return "", fmt.Errorf("node name is required")
	}
	if opts.ServerURL == "" {
		return "", fmt.Errorf("server URL is required")
	}
	if opts.ClientKeyPEM == "" {
		return "", fmt.Errorf("client key is required")
	}
	if opts.BootstrapURL == "" {
		opts.BootstrapURL = DefaultBootstrapURL
	}
	if opts.Environment == "" {
		opts.Environment = "_default"
	}
	firstBoot, err := firstBootJSON(opts)
	if err != nil {
		return "", err
	}
	clientRB := clientRB(opts)
	prefix := ""
	if opts.Sudo {
		prefix = "sudo "
	}
	installArgs := ""
	if opts.BootstrapVersion != "" {
		installArgs = " -v " + shellQuote(opts.BootstrapVersion)
	}
	commands := []string{
		"set -e",
		"curl -L " + shellQuote(opts.BootstrapURL) + " | " + prefix + "bash -s --" + installArgs,
		prefix + "mkdir -p /etc/cinc",
		writeFileCommand(prefix, "/etc/cinc/client.pem", opts.ClientKeyPEM),
		prefix + "chmod 0600 /etc/cinc/client.pem",
		writeFileCommand(prefix, "/etc/cinc/client.rb", clientRB),
		writeFileCommand(prefix, "/etc/cinc/first-boot.json", string(firstBoot)),
		prefix + "cinc-client -j /etc/cinc/first-boot.json",
	}
	return strings.Join(commands, "\n"), nil
}

func firstBootJSON(opts BootstrapOptions) ([]byte, error) {
	doc := map[string]any{}
	if len(opts.RunList) > 0 {
		doc["run_list"] = opts.RunList
	}
	if opts.Environment != "" {
		doc["chef_environment"] = opts.Environment
	}
	if opts.PolicyName != "" {
		doc["policy_name"] = opts.PolicyName
	}
	if opts.PolicyGroup != "" {
		doc["policy_group"] = opts.PolicyGroup
	}
	return json.MarshalIndent(doc, "", "  ")
}

func clientRB(opts BootstrapOptions) string {
	return fmt.Sprintf("chef_server_url %q\nnode_name %q\nclient_key %q\n", opts.ServerURL, opts.NodeName, "/etc/cinc/client.pem")
}

func writeFileCommand(prefix, path, content string) string {
	return "cat <<'CINC_EOF' | " + prefix + "tee " + shellQuote(path) + " >/dev/null\n" + content + "\nCINC_EOF"
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
