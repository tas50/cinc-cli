package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/printer"
)

// newClientCmd builds the `cinc client` command group.
func newClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Manage API clients on the Cinc/Chef Server",
	}
	cmd.AddCommand(newClientListCmd())
	cmd.AddCommand(newClientCreateCmd())
	cmd.AddCommand(newClientDeleteCmd())
	return cmd
}

// newClientCreateCmd builds the `cinc client create <name>` command.
//
// The server generates an RSA key pair and returns the private key in
// the response. By default the private key is streamed to stdout so it
// can be piped into a file; `--key-file` writes it to disk instead.
// `--public-key` lets the caller supply their own public key, in which
// case the server does not generate a key pair and no private key is
// returned.
func newClientCreateCmd() *cobra.Command {
	var (
		validator     bool
		keyFile       string
		publicKeyFile string
	)
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an API client on the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			req := &cinc.APIClient{Name: args[0], Validator: validator}
			if publicKeyFile != "" {
				pem, err := os.ReadFile(publicKeyFile)
				if err != nil {
					return fmt.Errorf("cinc: read public key: %w", err)
				}
				req.ChefKey.PublicKey = string(pem)
			}
			created, _, err := c.Clients.Create(cmd.Context(), req)
			if err != nil {
				return err
			}
			return emitClientCreateResult(cmd, req.Name, created, keyFile)
		},
	}
	cmd.Flags().BoolVar(&validator, "validator", false, "create a validator client")
	cmd.Flags().StringVarP(&keyFile, "key-file", "f", "", "write the generated private key to this file instead of stdout")
	cmd.Flags().StringVar(&publicKeyFile, "public-key", "", "path to a PEM public key; the server will not generate a key pair")
	return cmd
}

// emitClientCreateResult renders the response from a successful client
// create. When the server returns a private key, it is written to
// keyFile if given, otherwise streamed to stdout (with a trailing
// newline if the PEM does not already end in one). When no private key
// is returned — the BYO public key path — the command prints a single
// confirmation line.
func emitClientCreateResult(cmd *cobra.Command, name string, created *cinc.APIClient, keyFile string) error {
	out := cmd.OutOrStdout()
	priv := created.ChefKey.PrivateKey
	if priv == "" {
		fmt.Fprintf(out, "Created client %q\n", name)
		return nil
	}
	if keyFile != "" {
		if err := os.WriteFile(keyFile, []byte(priv), 0o600); err != nil {
			return fmt.Errorf("cinc: write key file: %w", err)
		}
		fmt.Fprintf(out, "Created client %q (key written to %s)\n", name, keyFile)
		return nil
	}
	if _, err := fmt.Fprint(out, priv); err != nil {
		return err
	}
	if !strings.HasSuffix(priv, "\n") {
		fmt.Fprintln(out)
	}
	return nil
}

// newClientDeleteCmd builds the `cinc client delete <name>` command.
func newClientDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an API client from the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			name := args[0]
			if _, err := c.Clients.Delete(cmd.Context(), name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted client %q\n", name)
			return nil
		},
	}
}

// newClientListCmd builds the `cinc client list` command.
func newClientListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List API clients on the server",
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
			names, err := fetchClientNames(cmd.Context(), c)
			if err != nil {
				return err
			}
			return printer.New(cmd.OutOrStdout(), format).List(names)
		},
	}
}

// fetchClientNames returns the sorted names of every API client on the
// server.
func fetchClientNames(ctx context.Context, c *cinc.Client) ([]string, error) {
	index, _, err := c.Clients.List(ctx)
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
