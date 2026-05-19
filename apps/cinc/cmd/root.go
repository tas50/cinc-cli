package cmd

import "github.com/spf13/cobra"

// newRootCmd builds the root `cinc` command and registers its
// subcommands.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "cinc",
		Short:         "cinc is a unified command-line tool for Chef/Cinc Infra",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newVersionCmd())

	return root
}

// Execute builds and runs the root command. It is the single entry point
// called by main().
func Execute() error {
	return newRootCmd().Execute()
}
