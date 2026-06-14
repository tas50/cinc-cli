package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build metadata. These are overridden at build time via -ldflags; the
// defaults apply to local/development builds.
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

// versionInfo describes the build and runtime details reported by
// `cinc version`.
type versionInfo struct {
	Version   string
	Commit    string
	BuildDate string
	GoVersion string
	Platform  string
}

// newVersionInfo collects the build metadata and the current runtime
// details into a versionInfo.
func newVersionInfo() versionInfo {
	return versionInfo{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// String renders the version details as human-readable output.
func (v versionInfo) String() string {
	return fmt.Sprintf(
		"cinc %s\n  commit:    %s\n  built:     %s\n  go:        %s\n  platform:  %s\n",
		v.Version, v.Commit, v.BuildDate, v.GoVersion, v.Platform,
	)
}

// newVersionCmd builds the `cinc version` command.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print cinc version information",
		Example: "Print the cinc version, commit, and build date.\n" +
			"cinc version",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(cmd.OutOrStdout(), newVersionInfo().String())
			return nil
		},
	}
}
