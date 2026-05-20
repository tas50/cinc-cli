package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tas50/cinc-cli/cli/config"
)

// newConfigureCmd builds `cinc configure`, the local workstation credentials
// setup command. It writes TOML credentials only; Cinc does not emit Ruby
// config.rb/client.rb files.
func newConfigureCmd() *cobra.Command {
	var (
		serverURL     string
		clientName    string
		clientKey     string
		sslVerifyMode string
		skipKeyCheck  bool
	)
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Create or update a local credentials profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if serverURL == "" {
				return fmt.Errorf("cinc: --server-url is required")
			}
			if clientName == "" {
				return fmt.Errorf("cinc: --client-name is required")
			}
			if clientKey == "" {
				return fmt.Errorf("cinc: --client-key is required")
			}
			if !skipKeyCheck {
				if err := requireReadableFile(clientKey); err != nil {
					return err
				}
			}
			profile, err := config.NewProfile(serverURL, clientName, clientKey, sslVerifyMode)
			if err != nil {
				return err
			}
			cfgPath, err := configPathForCommand(cmd)
			if err != nil {
				return err
			}
			profileName := profileNameForCommand(cmd)
			if err := config.WriteProfile(cfgPath, profileName, profile); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote credentials profile %q to %s\n", profileName, cfgPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverURL, "server-url", "", "Chef/Cinc Server URL including /organizations/<org>")
	cmd.Flags().StringVar(&serverURL, "chef-server-url", "", "Chef Server URL including /organizations/<org>")
	cmd.Flags().StringVar(&serverURL, "cinc-server-url", "", "Cinc Server URL including /organizations/<org>")
	cmd.Flags().StringVar(&clientName, "client-name", "", "client name used to sign API requests")
	cmd.Flags().StringVar(&clientKey, "client-key", "", "path to the PEM private key for the client")
	cmd.Flags().StringVar(&sslVerifyMode, "ssl-verify-mode", "", "optional SSL verify mode such as :verify_peer or :verify_none")
	cmd.Flags().BoolVar(&skipKeyCheck, "skip-key-check", false, "write the profile without checking that --client-key exists")
	return cmd
}

func configPathForCommand(cmd *cobra.Command) (string, error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	if cfgPath != "" {
		return cfgPath, nil
	}
	return config.DefaultPath()
}

func profileNameForCommand(cmd *cobra.Command) string {
	profileName, _ := cmd.Flags().GetString("profile")
	if profileName != "" {
		return profileName
	}
	if profileName = os.Getenv("CINC_PROFILE"); profileName != "" {
		return profileName
	}
	if profileName = os.Getenv("CHEF_PROFILE"); profileName != "" {
		return profileName
	}
	return "default"
}

func requireReadableFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cinc: read client key: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("cinc: client key %s is a directory", path)
	}
	return nil
}
