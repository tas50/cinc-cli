package cmd

import (
	"context"
	"fmt"
	"slices"

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
	cmd.AddCommand(newClientDeleteCmd())
	return cmd
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
