package cmd

import (
	"context"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/printer"
)

// newDataBagCmd builds the `cinc data-bag` command group.
func newDataBagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data-bag",
		Short: "Manage data bags on the Cinc/Chef Server",
	}
	cmd.AddCommand(newDataBagListCmd())
	cmd.AddCommand(newDataBagDeleteCmd())
	return cmd
}

// newDataBagDeleteCmd builds the `cinc data-bag delete <name>` command.
func newDataBagDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a data bag from the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			name := args[0]
			if _, err := c.DataBags.Delete(cmd.Context(), name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted data bag %q\n", name)
			return nil
		},
	}
}

// newDataBagListCmd builds the `cinc data-bag list` command.
func newDataBagListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List data bags on the server",
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
			names, err := fetchDataBagNames(cmd.Context(), c)
			if err != nil {
				return err
			}
			return printer.New(cmd.OutOrStdout(), format).List(names)
		},
	}
}

// fetchDataBagNames returns the sorted names of every data bag on the
// server.
func fetchDataBagNames(ctx context.Context, c *cinc.Client) ([]string, error) {
	index, _, err := c.DataBags.List(ctx)
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
