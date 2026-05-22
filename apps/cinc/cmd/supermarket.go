package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tas50/cinc-cli/cli/printer"
	"github.com/tas50/cinc-cli/cli/supermarket"
	"github.com/tas50/cinc-cli/cli/supermarket/explore"
)

// newSupermarketCmd builds the `cinc supermarket` command group.
func newSupermarketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "supermarket",
		Short: "Manage cookbooks on Chef Supermarket",
	}
	cmd.AddCommand(newSupermarketShareCmd())
	cmd.AddCommand(newSupermarketExploreCmd())
	cmd.AddCommand(newSupermarketDownloadCmd())
	return cmd
}

// newSupermarketDownloadCmd builds `cinc supermarket download`.
// Like `explore`, this hits only anonymous endpoints, so we never
// load a profile or key here.
func newSupermarketDownloadCmd() *cobra.Command {
	var (
		file  string
		force bool
		site  string
	)
	cmd := &cobra.Command{
		Use:   "download <cookbook> [version]",
		Short: "Download a cookbook tarball from Chef Supermarket",
		Long: "Downloads a cookbook from Chef Supermarket and writes it to disk\n" +
			"as a gzipped tarball. The version defaults to the latest published\n" +
			"version. By default the tarball lands at ./<cookbook>-<version>.tar.gz;\n" +
			"pass --file to choose a target file or directory.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			opts := supermarket.DownloadOptions{
				Cookbook: args[0],
				File:     file,
				Force:    force,
			}
			if len(args) == 2 {
				opts.Version = args[1]
			}
			client, err := supermarket.NewAnonymous(site)
			if err != nil {
				return err
			}
			result, err := client.Download(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if format == printer.FormatJSON {
				return printer.New(cmd.OutOrStdout(), format).Value(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %s %s to %s\n", result.Cookbook, result.Version, result.File)
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "output file or directory (default: ./<cookbook>-<version>.tar.gz)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the output file if it already exists")
	cmd.Flags().StringVar(&site, "supermarket-site", "", "URL of the Chef Supermarket site (default: https://supermarket.chef.io)")
	return cmd
}

// newSupermarketExploreCmd builds the `cinc supermarket explore` TUI.
// It needs no credentials — every endpoint it touches is anonymous —
// so we never run the first-run flow or load a profile here.
func newSupermarketExploreCmd() *cobra.Command {
	var site string
	cmd := &cobra.Command{
		Use:   "explore",
		Short: "Browse Chef Supermarket cookbooks in a terminal UI",
		Long: "Launches an interactive terminal UI for browsing cookbooks on Chef\n" +
			"Supermarket. Move with arrow keys, press / to search, press d/u/a to\n" +
			"sort by Downloads, Updated, or Alphabetical, enter for full details,\n" +
			"and q to quit.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return explore.Run(cmd.Context(), explore.Options{
				Site:   site,
				Stdin:  cmd.InOrStdin(),
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
		},
	}
	cmd.Flags().StringVar(&site, "supermarket-site", "", "URL of the Chef Supermarket site (default: https://supermarket.chef.io)")
	return cmd
}

// newSupermarketShareCmd builds the `cinc supermarket share <cookbook>` command.
func newSupermarketShareCmd() *cobra.Command {
	var (
		cookbookPath string
		site         string
		dryRun       bool
		noChefignore bool
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
				Cookbook:       args[0],
				CookbookPath:   cookbookPath,
				DryRun:         dryRun,
				SkipChefignore: noChefignore,
			}
			if len(args) == 2 {
				opts.Category = args[1]
			}
			var result supermarket.ShareResult
			if dryRun {
				result, err = supermarket.DryRun(opts)
			} else {
				var profileErr error
				profile, profileErr := resolveSupermarketProfile(cmd)
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
	cmd.Flags().StringVar(&site, "supermarket-site", "", "URL of the Chef Supermarket site (default: profile supermarket_site, then https://supermarket.chef.io)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "build the cookbook tarball without uploading it")
	cmd.Flags().BoolVar(&noChefignore, "no-chefignore", false, "do not exclude files matched by the cookbook's chefignore file")
	return cmd
}
