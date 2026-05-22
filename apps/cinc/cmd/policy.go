package cmd

import (
	"context"
	"slices"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/printer"
)

// newPolicyCmd builds the `cinc policy` command group.
func newPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage Policyfile policies on the Cinc Server",
	}
	cmd.AddCommand(newPolicyListCmd())
	return cmd
}

// newPolicyListCmd builds the `cinc policy list` command.
func newPolicyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List policies on the server",
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
			names, err := fetchPolicyNames(cmd.Context(), c)
			if err != nil {
				return err
			}
			return printer.New(cmd.OutOrStdout(), format).List(names)
		},
	}
}

// fetchPolicyNames returns the sorted names of every policy on the
// server. The list response carries per-policy revision metadata; the
// list verb only enumerates names.
func fetchPolicyNames(ctx context.Context, c *cinc.Client) ([]string, error) {
	index, _, err := c.Policies.List(ctx)
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
