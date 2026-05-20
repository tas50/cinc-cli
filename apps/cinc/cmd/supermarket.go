package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tas50/cinc-cli/cli/printer"
	"github.com/tas50/cinc-cli/cli/supermarket"
)

// newSupermarketCmd builds the `cinc supermarket` command group.
func newSupermarketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "supermarket",
		Short: "Manage cookbooks on Chef Supermarket",
	}
	cmd.AddCommand(newSupermarketShareCmd())
	return cmd
}

// newSupermarketShareCmd builds the `cinc supermarket share <cookbook>` command.
func newSupermarketShareCmd() *cobra.Command {
	var (
		cookbookPath string
		site         string
		dryRun       bool
	)
	cmd := &cobra.Command{
		Use:   "share <cookbook> [category]",
		Short: "Share a cookbook on Chef Supermarket",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			opts := supermarket.ShareOptions{
				Cookbook:     args[0],
				CookbookPath: cookbookPath,
				DryRun:       dryRun,
			}
			if len(args) == 2 {
				opts.Category = args[1]
			}
			var result supermarket.ShareResult
			if dryRun {
				result, err = supermarket.DryRun(opts)
			} else {
				var profileErr error
				profile, profileErr := resolveProfile(cmd)
				if profileErr != nil {
					return profileErr
				}
				client, clientErr := supermarket.New(profile, site)
				if clientErr != nil {
					return clientErr
				}
				result, err = client.Share(cmd.Context(), opts)
			}
			if err != nil {
				return err
			}
			if format == printer.FormatJSON {
				return printer.New(cmd.OutOrStdout(), format).Value(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Making tarball %s\n", result.Tarball)
			if dryRun {
				for _, file := range result.Files {
					fmt.Fprintln(cmd.OutOrStdout(), file)
				}
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Upload complete")
			return nil
		},
	}
	cmd.Flags().StringVar(&cookbookPath, "cookbook-path", "", "directory or path list containing cookbooks (default current directory)")
	cmd.Flags().StringVar(&site, "supermarket-site", supermarket.DefaultSite, "URL of the Chef Supermarket site")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "build the cookbook tarball without uploading it")
	return cmd
}
