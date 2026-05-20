package cmd

import (
	"os"

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
	profile, err := resolveProfile(cmd)
	if err != nil {
		return nil, err
	}
	return client.New(profile)
}

// resolveProfile reads the selected profile from the --config and --profile
// flags without constructing a server client.
func resolveProfile(cmd *cobra.Command) (config.Profile, error) {
	cfgPath, err := configFilePath(cmd)
	if err != nil {
		return config.Profile{}, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return config.Profile{}, err
	}

	profileName, _ := cmd.Flags().GetString("profile")
	profile, err := cfg.Profile(profileName)
	if err != nil {
		return config.Profile{}, err
	}
	return profile, nil
}

// resolveSupermarketProfile prefers the explicit --profile or environment
// profile when present. Otherwise it uses the conventional [supermarket]
// profile, falling back to [default] for existing credentials files.
func resolveSupermarketProfile(cmd *cobra.Command) (config.Profile, error) {
	cfgPath, err := configFilePath(cmd)
	if err != nil {
		return config.Profile{}, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return config.Profile{}, err
	}

	if profileName, _ := cmd.Flags().GetString("profile"); profileName != "" {
		return cfg.Profile(profileName)
	}
	if profileName := os.Getenv("CINC_PROFILE"); profileName != "" {
		return cfg.Profile(profileName)
	}
	if profileName := os.Getenv("CHEF_PROFILE"); profileName != "" {
		return cfg.Profile(profileName)
	}
	if profile, err := cfg.Profile("supermarket"); err == nil {
		return profile, nil
	}
	return cfg.Profile("default")
}

func configFilePath(cmd *cobra.Command) (string, error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	if cfgPath != "" {
		return cfgPath, nil
	}
	return config.DefaultPath()
}
