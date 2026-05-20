package cmd

import (
	"context"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/printer"
)

// newNodeCmd builds the `cinc node` command group.
func newNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage nodes on the Cinc/Chef Server",
	}
	cmd.AddCommand(newNodeListCmd())
	cmd.AddCommand(newNodeDeleteCmd())
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
