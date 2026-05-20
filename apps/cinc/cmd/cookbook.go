package cmd

import (
	"context"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/printer"
)

// newCookbookCmd builds the `cinc cookbook` command group.
func newCookbookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cookbook",
		Short: "Manage cookbooks on the Cinc/Chef Server",
	}
	cmd.AddCommand(newCookbookListCmd())
	cmd.AddCommand(newCookbookDeleteCmd())
	return cmd
}

// newCookbookDeleteCmd builds the `cinc cookbook delete <name> <version>`
// command. The server identifies a cookbook by name and version, so both
// are required.
func newCookbookDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name> <version>",
		Short: "Delete a cookbook version from the server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			name, version := args[0], args[1]
			if _, err := c.Cookbooks.Delete(cmd.Context(), name, version); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted cookbook %q version %s\n", name, version)
			return nil
		},
	}
}

// newCookbookListCmd builds the `cinc cookbook list` command.
func newCookbookListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cookbooks on the server",
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
			names, err := fetchCookbookNames(cmd.Context(), c)
			if err != nil {
				return err
			}
			return printer.New(cmd.OutOrStdout(), format).List(names)
		},
	}
}

// fetchCookbookNames returns the sorted names of every cookbook on the
// server. Cookbooks.List returns version metadata per entry; the list
// verb only enumerates names.
func fetchCookbookNames(ctx context.Context, c *cinc.Client) ([]string, error) {
	index, _, err := c.Cookbooks.List(ctx)
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
