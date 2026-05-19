package cmd

import (
	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/client"
	"github.com/tas50/cinc-cli/cli/config"
	"github.com/tas50/cinc-cli/cli/printer"
)

// resolveFormat reads and validates the --format flag.
func resolveFormat(cmd *cobra.Command) (printer.Format, error) {
	name, _ := cmd.Flags().GetString("format")
	return printer.ParseFormat(name)
}

// resolveClient builds a server client from the --config and --profile
// flags. An empty --config falls back to the default config path.
func resolveClient(cmd *cobra.Command) (*cinc.Client, error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	if cfgPath == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return nil, err
		}
		cfgPath = p
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	profileName, _ := cmd.Flags().GetString("profile")
	profile, err := cfg.Profile(profileName)
	if err != nil {
		return nil, err
	}
	return client.New(profile)
}
