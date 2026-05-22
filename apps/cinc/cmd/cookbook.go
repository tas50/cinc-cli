package cmd

import (
	"context"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	localcookbook "github.com/tas50/cinc-cli/cli/cookbook"
	"github.com/tas50/cinc-cli/cli/printer"
)

// newCookbookCmd builds the `cinc cookbook` command group.
func newCookbookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cookbook",
		Short: "Manage cookbooks on the Cinc Server",
	}
	cmd.AddCommand(newCookbookListCmd())
	cmd.AddCommand(newCookbookShowCmd())
	cmd.AddCommand(newCookbookDeleteCmd())
	cmd.AddCommand(newCookbookUploadCmd())
	return cmd
}

// newCookbookShowCmd builds the `cinc cookbook show <name> [version]`
// command. With no version the command resolves the special "_latest"
// sentinel that the Chef Server exposes for the highest semver.
func newCookbookShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name> [version]",
		Short: "Show a cookbook version manifest",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			version := "_latest"
			if len(args) == 2 {
				version = args[1]
			}
			cb, _, err := c.Cookbooks.Get(cmd.Context(), args[0], version)
			if err != nil {
				return err
			}
			return printer.New(cmd.OutOrStdout(), format).Value(cb)
		},
	}
}

// newCookbookUploadCmd builds the `cinc cookbook upload <name>...` command.
func newCookbookUploadCmd() *cobra.Command {
	var cookbookPath string
	cmd := &cobra.Command{
		Use:   "upload <name>...",
		Short: "Upload cookbook versions to the Cinc Server",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			results := make([]cookbookUploadResult, 0, len(args))
			for _, name := range args {
				dir, err := localcookbook.Locate(name, cookbookPath)
				if err != nil {
					return err
				}
				version, err := localcookbook.ReadVersion(dir)
				if err != nil {
					return err
				}
				cb, err := localcookbook.UploadableFromDir(dir, version)
				if err != nil {
					return err
				}
				if err := c.Cookbooks.Upload(cmd.Context(), cb); err != nil {
					return err
				}
				results = append(results, cookbookUploadResult{
					Cookbook: name, Version: version, Uploaded: true,
				})
			}
			if format == printer.FormatJSON {
				return printer.New(cmd.OutOrStdout(), format).Value(results)
			}
			for _, result := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "Uploaded cookbook %q version %s\n", result.Cookbook, result.Version)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cookbookPath, "cookbook-path", "", "directory or path list containing cookbooks (default current directory)")
	return cmd
}

type cookbookUploadResult struct {
	Cookbook string `json:"cookbook"`
	Version  string `json:"version"`
	Uploaded bool   `json:"uploaded"`
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
