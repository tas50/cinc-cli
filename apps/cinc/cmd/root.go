package cmd

import "github.com/spf13/cobra"

// newRootCmd builds the root `cinc` command and registers its
// subcommands.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cinc",
		Short: "cinc is a unified command-line tool for Chef/Cinc Infra",
		// Errors are still printed by cobra; only the usage dump is
		// suppressed so a runtime failure shows just the error.
		SilenceUsage: true,
	}

	flags := root.PersistentFlags()
	flags.String("config", "", "path to the cinc config file (default ~/.cinc/config.toml)")
	flags.String("profile", "", "configuration profile to use (default: the config's default_profile)")
	flags.String("format", "human", "output format: human or json")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newNodeCmd())

	return root
}

// Execute builds and runs the root command. It is the single entry point
// called by main().
func Execute() error {
	return newRootCmd().Execute()
}
