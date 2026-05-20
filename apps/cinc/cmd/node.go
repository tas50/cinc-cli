package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/components"
	"github.com/tas50/cinc-cli/cli/printer"
	"github.com/tas50/cinc-cli/cli/remote"
)

var nodeRemoteRunner remote.Runner = remote.NativeRunner{}

// newNodeCmd builds the `cinc node` command group.
func newNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage nodes on the Cinc/Chef Server",
	}
	cmd.AddCommand(newNodeListCmd())
	cmd.AddCommand(newNodeDeleteCmd())
	cmd.AddCommand(newNodeSSHCmd())
	cmd.AddCommand(newNodeBootstrapCmd())
	return cmd
}

type nodeSSHFlags struct {
	user         string
	password     string
	identityFile string
	port         int
	timeout      time.Duration
	useAgent     bool
	verifyHost   bool
	noVerifyHost bool
	manualList   bool
	attribute    string
	concurrency  int
	exitOnError  bool
}

func newNodeSSHCmd() *cobra.Command {
	flags := nodeSSHFlags{port: 22, timeout: 30 * time.Second, useAgent: true, verifyHost: true, attribute: "fqdn", concurrency: 10}
	cmd := &cobra.Command{
		Use:   "ssh [search-query] [command]",
		Short: "Run an SSH command on nodes",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			query, sshCommand := argAt(args, 0), argAt(args, 1)
			if err := promptNodeSSH(cmd, &query, &sshCommand, &flags); err != nil {
				return err
			}
			targets, err := nodeSSHTargets(cmd, query, flags)
			if err != nil {
				return err
			}
			results := remote.RunMany(cmd.Context(), nodeRemoteRunner, targets, sshCommand, remoteOptions(flags), flags.concurrency, flags.exitOnError)
			if format == printer.FormatJSON {
				if err := printer.New(cmd.OutOrStdout(), format).Value(results); err != nil {
					return err
				}
			} else {
				printRemoteResults(cmd, results)
			}
			if failed := countRemoteFailures(results); failed > 0 {
				return fmt.Errorf("node ssh failed on %d host(s)", failed)
			}
			return nil
		},
	}
	addNodeSSHFlags(cmd, &flags)
	cmd.Flags().BoolVar(&flags.manualList, "manual-list", false, "treat search query as a space-separated host list")
	cmd.Flags().StringVar(&flags.attribute, "attribute", "fqdn", "node attribute used as the SSH host")
	cmd.Flags().IntVar(&flags.concurrency, "concurrency", 10, "maximum concurrent SSH sessions")
	cmd.Flags().BoolVar(&flags.exitOnError, "exit-on-error", false, "stop launching new SSH sessions after the first failure")
	return cmd
}

type nodeBootstrapFlags struct {
	nodeSSHFlags
	nodeName         string
	runList          string
	environment      string
	policyName       string
	policyGroup      string
	sudo             bool
	bootstrapVersion string
	bootstrapURL     string
	dryRun           bool
}

func newNodeBootstrapCmd() *cobra.Command {
	flags := nodeBootstrapFlags{
		nodeSSHFlags: nodeSSHFlags{port: 22, timeout: 30 * time.Second, useAgent: true, verifyHost: true},
		environment:  "_default",
		sudo:         true,
		bootstrapURL: remote.DefaultBootstrapURL,
	}
	cmd := &cobra.Command{
		Use:   "bootstrap [target]",
		Short: "Bootstrap a node with Cinc Client over SSH",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := argAt(args, 0)
			if err := promptNodeBootstrap(cmd, &target, &flags); err != nil {
				return err
			}
			if flags.nodeName == "" {
				flags.nodeName = target
			}
			if err := validateBootstrapFlags(flags); err != nil {
				return err
			}
			profile, err := resolveProfile(cmd)
			if err != nil {
				return err
			}
			var privateKey string
			if !flags.dryRun {
				c, err := resolveClient(cmd)
				if err != nil {
					return err
				}
				publicKey := ""
				privateKey, publicKey, err = remote.GenerateClientKeyPair()
				if err != nil {
					return fmt.Errorf("bootstrap: generate client key: %w", err)
				}
				req := &cinc.APIClient{Name: flags.nodeName}
				req.ChefKey.PublicKey = publicKey
				if _, _, err := c.Clients.Create(cmd.Context(), req); err != nil {
					return err
				}
			} else {
				privateKey = "DRY-RUN-CLIENT-KEY"
			}
			bootstrapCommand, err := remote.BootstrapCommand(remote.BootstrapOptions{
				NodeName:         flags.nodeName,
				ServerURL:        strings.TrimRight(profile.ServerURL, "/") + "/organizations/" + profile.Org,
				ClientKeyPEM:     privateKey,
				RunList:          splitCSV(flags.runList),
				Environment:      flags.environment,
				PolicyName:       flags.policyName,
				PolicyGroup:      flags.policyGroup,
				Sudo:             flags.sudo,
				BootstrapURL:     flags.bootstrapURL,
				BootstrapVersion: flags.bootstrapVersion,
			})
			if err != nil {
				return err
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), bootstrapCommand)
				return nil
			}
			result := nodeRemoteRunner.Run(cmd.Context(), remote.Target{Host: target}, bootstrapCommand, remoteOptions(flags.nodeSSHFlags))
			if result.ExitCode != 0 {
				return fmt.Errorf("bootstrap failed on %s: %s; client %q was created and may need cleanup before retry", target, firstNonEmpty(result.Error, result.Stderr), flags.nodeName)
			}
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			if format == printer.FormatJSON {
				return printer.New(cmd.OutOrStdout(), format).Value(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bootstrapped node %q on %s\n", flags.nodeName, target)
			return nil
		},
	}
	addNodeSSHFlags(cmd, &flags.nodeSSHFlags)
	cmd.Flags().StringVar(&flags.nodeName, "node-name", "", "node name to configure on the Cinc Server (default target host)")
	cmd.Flags().StringVar(&flags.runList, "run-list", "", "comma-separated first run-list")
	cmd.Flags().StringVar(&flags.environment, "environment", "_default", "first run environment")
	cmd.Flags().StringVar(&flags.policyName, "policy-name", "", "policyfile name for first run")
	cmd.Flags().StringVar(&flags.policyGroup, "policy-group", "", "policy group for first run")
	cmd.Flags().BoolVar(&flags.sudo, "sudo", true, "run bootstrap commands through sudo")
	cmd.Flags().StringVar(&flags.bootstrapVersion, "bootstrap-version", "", "Cinc Client version to install")
	cmd.Flags().StringVar(&flags.bootstrapURL, "bootstrap-url", remote.DefaultBootstrapURL, "Cinc install script URL")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "print the bootstrap script without creating a client or connecting")
	return cmd
}

// newNodeListCmd builds the `cinc node list` command.
func newNodeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List nodes on the server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			names, err := fetchNodeNames(cmd.Context(), c)
			if err != nil {
				return err
			}
			return printer.New(cmd.OutOrStdout(), format).List(names)
		},
	}
}

// newNodeDeleteCmd builds the `cinc node delete <name>` command.
func newNodeDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a node from the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			name := args[0]
			if _, err := c.Nodes.Delete(cmd.Context(), name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted node %q\n", name)
			return nil
		},
	}
}

// fetchNodeNames returns the sorted names of every node on the server.
func fetchNodeNames(ctx context.Context, c *cinc.Client) ([]string, error) {
	index, _, err := c.Nodes.List(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(index))
	for name := range index {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

func addNodeSSHFlags(cmd *cobra.Command, flags *nodeSSHFlags) {
	cmd.Flags().StringVar(&flags.user, "ssh-user", "", "SSH user")
	cmd.Flags().StringVar(&flags.password, "ssh-password", "", "SSH password")
	cmd.Flags().StringVar(&flags.identityFile, "ssh-identity-file", "", "SSH identity file")
	cmd.Flags().IntVar(&flags.port, "ssh-port", 22, "SSH port")
	cmd.Flags().DurationVar(&flags.timeout, "ssh-timeout", 30*time.Second, "SSH connection timeout")
	cmd.Flags().BoolVar(&flags.useAgent, "ssh-agent", true, "use SSH agent authentication")
	cmd.Flags().BoolVar(&flags.verifyHost, "host-key-verify", true, "verify SSH host keys using known_hosts")
	cmd.Flags().BoolVar(&flags.noVerifyHost, "no-host-key-verify", false, "disable SSH host key verification")
}

func promptNodeSSH(cmd *cobra.Command, query, sshCommand *string, flags *nodeSSHFlags) error {
	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()
	var err error
	if *query == "" {
		*query, err = components.PromptWithDefault(reader, out, "Search query or host list", *query)
		if err != nil {
			return err
		}
	}
	if *sshCommand == "" {
		*sshCommand, err = components.PromptWithDefault(reader, out, "SSH command", *sshCommand)
		if err != nil {
			return err
		}
	}
	if flags.user == "" {
		flags.user, err = components.PromptWithDefault(reader, out, "SSH user", flags.user)
		if err != nil {
			return err
		}
	}
	if *query == "" || *sshCommand == "" || flags.user == "" {
		return fmt.Errorf("node ssh requires search query, command, and --ssh-user")
	}
	return nil
}

func promptNodeBootstrap(cmd *cobra.Command, target *string, flags *nodeBootstrapFlags) error {
	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()
	var err error
	if *target == "" {
		*target, err = components.PromptWithDefault(reader, out, "Target host", *target)
		if err != nil {
			return err
		}
	}
	if flags.user == "" {
		flags.user, err = components.PromptWithDefault(reader, out, "SSH user", flags.user)
		if err != nil {
			return err
		}
	}
	if flags.nodeName == "" {
		flags.nodeName, err = components.PromptWithDefault(reader, out, "Node name", *target)
		if err != nil {
			return err
		}
	}
	if *target == "" || flags.user == "" || flags.nodeName == "" {
		return fmt.Errorf("node bootstrap requires target, --ssh-user, and node name")
	}
	return nil
}

func nodeSSHTargets(cmd *cobra.Command, query string, flags nodeSSHFlags) ([]remote.Target, error) {
	if flags.manualList {
		hosts := strings.Fields(query)
		targets := make([]remote.Target, 0, len(hosts))
		for _, host := range hosts {
			targets = append(targets, remote.Target{Host: host})
		}
		return targets, nil
	}
	c, err := resolveClient(cmd)
	if err != nil {
		return nil, err
	}
	rows, err := c.Search.SearchAll(cmd.Context(), "node", expandNodeSSHQuery(query))
	if err != nil {
		return nil, err
	}
	targets := make([]remote.Target, 0, len(rows))
	for _, row := range rows {
		host, err := searchRowAttribute(row, flags.attribute)
		if err != nil {
			return nil, err
		}
		if host != "" {
			targets = append(targets, remote.Target{Host: host})
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("node ssh found no hosts for query %q", query)
	}
	return targets, nil
}

func expandNodeSSHQuery(query string) string {
	if strings.Contains(query, ":") {
		return query
	}
	return "tags:*" + query + "* OR roles:*" + query + "* OR fqdn:*" + query + "* OR addresses:*" + query + "*"
}

func searchRowAttribute(row json.RawMessage, attr string) (string, error) {
	var data any
	if err := json.Unmarshal(row, &data); err != nil {
		return "", err
	}
	for _, path := range candidateAttributePaths(attr) {
		if value, ok := lookupAttribute(data, path); ok {
			return attributeString(value), nil
		}
	}
	return "", fmt.Errorf("search row missing SSH attribute %q", attr)
}

func candidateAttributePaths(attr string) [][]string {
	if strings.Contains(attr, ".") {
		return [][]string{strings.Split(attr, ".")}
	}
	return [][]string{{attr}, {"automatic", attr}, {"normal", attr}, {"default", attr}, {"override", attr}}
}

func lookupAttribute(data any, path []string) (any, bool) {
	current := data
	for _, part := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func attributeString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		if len(v) == 0 {
			return ""
		}
		return attributeString(v[0])
	default:
		return fmt.Sprint(v)
	}
}

func remoteOptions(flags nodeSSHFlags) remote.SSHOptions {
	return remote.SSHOptions{
		User: flags.user, Password: flags.password, IdentityFile: flags.identityFile,
		Port: flags.port, Timeout: flags.timeout, UseAgent: flags.useAgent, VerifyHost: flags.verifyHost && !flags.noVerifyHost,
	}
}

func printRemoteResults(cmd *cobra.Command, results []remote.CommandResult) {
	for _, result := range results {
		writePrefixedLines(cmd.OutOrStdout(), result.Host, result.Stdout)
		writePrefixedLines(cmd.OutOrStdout(), result.Host, result.Stderr)
		if result.Error != "" && result.Stdout == "" && result.Stderr == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", result.Host, result.Error)
		}
	}
}

func writePrefixedLines(out interface{ Write([]byte) (int, error) }, host, text string) {
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if line != "" {
			fmt.Fprintf(out, "%s\t%s\n", host, line)
		}
	}
}

func countRemoteFailures(results []remote.CommandResult) int {
	var failed int
	for _, result := range results {
		if result.ExitCode != 0 {
			failed++
		}
	}
	return failed
}

func validateBootstrapFlags(flags nodeBootstrapFlags) error {
	if (flags.policyName == "") != (flags.policyGroup == "") {
		return fmt.Errorf("--policy-name and --policy-group must be provided together")
	}
	if flags.policyName != "" && flags.runList != "" {
		return fmt.Errorf("--run-list cannot be combined with policy bootstrap flags")
	}
	return nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func argAt(args []string, i int) string {
	if i >= len(args) {
		return ""
	}
	return args[i]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "unknown error"
}
