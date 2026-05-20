package cmd

import "github.com/spf13/cobra"

// newRootCmd builds the root `cinc` command and registers its
// subcommands.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cinc",
		Short: "cinc is a unified command-line tool for Cinc/Chef Infra",
		// Errors are still printed by cobra; only the usage dump is
		// suppressed so a runtime failure shows just the error.
		SilenceUsage: true,
	}

	flags := root.PersistentFlags()
	flags.String("config", "", "path to the cinc credentials file (default ~/.cinc/credentials)")
	flags.String("profile", "", "credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then \"default\")")
	flags.String("format", "human", "output format: human or json")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newConfigureCmd())
	root.AddCommand(newNodeCmd())
	root.AddCommand(newClientCmd())
	root.AddCommand(newRoleCmd())
	root.AddCommand(newEnvironmentCmd())
	root.AddCommand(newCookbookCmd())
	root.AddCommand(newDataBagCmd())
	root.AddCommand(newSupermarketCmd())

	return root
}

// Execute builds and runs the root command. It is the single entry point
// called by main().
func Execute() error {
	return newRootCmd().Execute()
}

// NewRootCmd returns a fresh root command tree. It exists so that
// out-of-tree tools (for example the doc generator under
// `tools/gendocs`) can walk the command tree without going through
// Execute. Tests inside this package should keep using the unexported
// newRootCmd.
func NewRootCmd() *cobra.Command {
	return newRootCmd()
}
