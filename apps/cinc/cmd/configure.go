package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tas50/cinc-cli/cli/config"
	"github.com/tas50/cinc-cli/cli/supermarket"
)

// newConfigureCmd builds `cinc configure`, the local workstation credentials
// setup command. It writes TOML credentials only; Cinc does not emit Ruby
// config.rb/client.rb files.
func newConfigureCmd() *cobra.Command {
	var (
		serverURL       string
		supermarketSite string
		clientName      string
		clientKey       string
		sslVerifyMode   string
		skipKeyCheck    bool
	)
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Create or update a local credentials profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfgPath, err := configPathForCommand(cmd)
			if err != nil {
				return err
			}
			profileName := profileNameForCommand(cmd)

			if !configureOptionsChanged(cmd) {
				answers, err := promptConfigure(cmd, configureDefaults{
					ConfigPath:      cfgPath,
					ProfileName:     configureProfileNameForCommand(cmd),
					SupermarketSite: supermarket.DefaultSite,
					ClientName:      defaultClientName(),
					ChefServerURL:   serverURL,
					SSLVerifyMode:   sslVerifyMode,
				})
				if err != nil {
					return err
				}
				cfgPath = answers.ConfigPath
				profileName = answers.ProfileName
				supermarketSite = answers.SupermarketSite
				clientName = answers.ClientName
				clientKey = answers.ClientKey
				serverURL = answers.ChefServerURL
				sslVerifyMode = answers.SSLVerifyMode
			} else if serverURL == "" && supermarketSite == "" {
				supermarketSite = supermarket.DefaultSite
			}
			if !configureProfileExplicit(cmd) && isPublicSupermarketProfile(serverURL, supermarketSite) {
				profileName = "supermarket"
			}

			if clientName == "" {
				return fmt.Errorf("cinc: --client-name is required")
			}
			if clientKey == "" {
				return fmt.Errorf("cinc: --client-key is required")
			}
			cfgPath, err = expandHome(cfgPath)
			if err != nil {
				return err
			}
			clientKey, err = expandHome(clientKey)
			if err != nil {
				return err
			}
			if !skipKeyCheck {
				if err := requireReadableFile(clientKey); err != nil {
					return err
				}
			}
			profile, err := config.NewProfile(serverURL, clientName, clientKey, sslVerifyMode, supermarketSite)
			if err != nil {
				return err
			}
			if err := config.WriteProfile(cfgPath, profileName, profile); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out)
			fmt.Fprintf(out, "Wrote credentials profile %q to %s\n", profileName, cfgPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverURL, "server-url", "", "Chef/Cinc Server URL including /organizations/<org>")
	cmd.Flags().StringVar(&serverURL, "chef-server-url", "", "Chef Server URL including /organizations/<org>")
	cmd.Flags().StringVar(&serverURL, "cinc-server-url", "", "Cinc Server URL including /organizations/<org>")
	cmd.Flags().StringVar(&supermarketSite, "supermarket-site", "", "Chef Supermarket URL for cookbook uploads")
	cmd.Flags().StringVar(&clientName, "client-name", "", "client name used to sign API requests")
	cmd.Flags().StringVar(&clientKey, "client-key", "", "path to the PEM private key for the client")
	cmd.Flags().StringVar(&sslVerifyMode, "ssl-verify-mode", "", "optional SSL verify mode such as :verify_peer or :verify_none")
	cmd.Flags().BoolVar(&skipKeyCheck, "skip-key-check", false, "write the profile without checking that --client-key exists")
	return cmd
}

type configureDefaults struct {
	ConfigPath      string
	ProfileName     string
	SupermarketSite string
	ClientName      string
	ClientKey       string
	ChefServerURL   string
	SSLVerifyMode   string
}

func promptConfigure(cmd *cobra.Command, defaults configureDefaults) (configureDefaults, error) {
	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Let's set up your Cinc credentials — press Enter on any prompt to accept the default.")
	fmt.Fprintln(out)

	var err error
	defaults.ConfigPath, err = promptWithDefault(reader, out, "Credentials file location", defaults.ConfigPath)
	if err != nil {
		return configureDefaults{}, err
	}
	defaults.ProfileName, err = promptWithDefault(reader, out, "Profile name", defaults.ProfileName)
	if err != nil {
		return configureDefaults{}, err
	}
	defaults.SupermarketSite, err = promptWithDefault(reader, out, "Supermarket site", defaults.SupermarketSite)
	if err != nil {
		return configureDefaults{}, err
	}
	defaults.ClientName, err = promptWithDefault(reader, out, "Client name", defaults.ClientName)
	if err != nil {
		return configureDefaults{}, err
	}
	if defaults.ClientKey == "" {
		defaults.ClientKey = defaultClientKey(defaults.ClientName)
	}
	defaults.ClientKey, err = promptWithDefault(reader, out, "Client key path", defaults.ClientKey)
	if err != nil {
		return configureDefaults{}, err
	}
	serverHost, err := promptWithDefault(reader, out, "Chef server host (optional, e.g. chef.example.com)", "")
	if err != nil {
		return configureDefaults{}, err
	}
	if serverHost != "" {
		serverOrg, err := promptWithDefault(reader, out, "Chef server organization", "")
		if err != nil {
			return configureDefaults{}, err
		}
		if serverOrg != "" {
			defaults.ChefServerURL = fmt.Sprintf("https://%s/organizations/%s", serverHost, serverOrg)
		}
	}
	if defaults.SSLVerifyMode == "" {
		defaults.SSLVerifyMode = ":verify_peer"
	}
	defaults.SSLVerifyMode, err = promptWithDefault(reader, out, "SSL verify mode", defaults.SSLVerifyMode)
	if err != nil {
		return configureDefaults{}, err
	}
	return defaults, nil
}

func promptWithDefault(reader *bufio.Reader, out io.Writer, label, defaultValue string) (string, error) {
	fmt.Fprintf(out, "%s [%s]: ", label, defaultValue)
	answer, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return defaultValue, nil
	}
	return answer, nil
}

func configureOptionsChanged(cmd *cobra.Command) bool {
	for _, name := range []string{
		"server-url",
		"chef-server-url",
		"cinc-server-url",
		"supermarket-site",
		"client-name",
		"client-key",
		"ssl-verify-mode",
		"skip-key-check",
	} {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func configureProfileNameForCommand(cmd *cobra.Command) string {
	if configureProfileExplicit(cmd) {
		return profileNameForCommand(cmd)
	}
	return "default"
}

func configureProfileExplicit(cmd *cobra.Command) bool {
	if profileName, _ := cmd.Flags().GetString("profile"); profileName != "" {
		return true
	}
	if os.Getenv("CINC_PROFILE") != "" {
		return true
	}
	if os.Getenv("CHEF_PROFILE") != "" {
		return true
	}
	return false
}

func isPublicSupermarketProfile(serverURL, supermarketSite string) bool {
	return strings.TrimRight(serverURL, "/") == supermarket.DefaultSite ||
		strings.TrimRight(supermarketSite, "/") == supermarket.DefaultSite
}

func defaultClientName() string {
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	return ""
}

func defaultClientKey(clientName string) string {
	if clientName == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", clientName+".pem")
}

func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cinc: locate home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
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
