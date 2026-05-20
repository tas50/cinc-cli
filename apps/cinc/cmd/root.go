package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/tas50/cinc-cli/cli/config"
)

// newRootCmd builds the root `cinc` command and registers its
// subcommands.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cinc",
		Short: "cinc is a unified command-line tool for Cinc/Chef Infra",
		// Errors are still printed by cobra; only the usage dump is
		// suppressed so a runtime failure shows just the error.
		SilenceUsage: true,
		// When invoked with no subcommand we still want a chance to
		// offer chef-credentials migration before falling back to the
		// help text. Subcommands keep their own behavior — this RunE
		// only fires for a bare `cinc` invocation.
		RunE: rootRunE,
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

// rootRunE handles a bare `cinc` invocation. If the default
// credentials file is missing it runs the first-run flow (chef
// migration when a knife config is present, otherwise an inline
// configure walk-through), then prints the usage help so the user
// still sees what commands are available. First-run failures are
// surfaced but do not block help.
func rootRunE(cmd *cobra.Command, _ []string) error {
	cincPath, err := config.DefaultPath()
	if err == nil {
		if _, err := os.Stat(cincPath); errors.Is(err, fs.ErrNotExist) {
			if _, runErr := offerFirstRun(cmd, cincPath); runErr != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), runErr)
			}
		}
	}
	return cmd.Help()
}
