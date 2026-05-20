package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/client"
	"github.com/tas50/cinc-cli/cli/config"
	"github.com/tas50/cinc-cli/cli/printer"
	"github.com/tas50/cinc-cli/cli/setup"
)

// migrateChef is the function used to migrate ~/.chef/credentials when the
// default cinc credentials file is missing. It is a package-level variable
// so tests can swap in a fake.
var migrateChef = setup.MigrateChef

// stdinIsTTY reports whether os.Stdin is attached to a character device.
// Swapped by tests that exercise the migration branch without a real TTY.
var stdinIsTTY = func() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

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
	cfg, err := loadCredentials(cmd)
	if err != nil {
		return config.Profile{}, err
	}
	profileName, _ := cmd.Flags().GetString("profile")
	return cfg.Profile(profileName)
}

// resolveSupermarketProfile prefers the explicit --profile or environment
// profile when present. Otherwise it uses the conventional [supermarket]
// profile, falling back to [default] for existing credentials files.
func resolveSupermarketProfile(cmd *cobra.Command) (config.Profile, error) {
	cfg, err := loadCredentials(cmd)
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

// loadCredentials returns the parsed credentials file the command was
// pointed at. If --config is empty (the default ~/.cinc/credentials
// path) and the file is missing, the user is offered a one-shot
// migration from ~/.chef/credentials before the load is retried. An
// explicit --config pointing at a missing file is surfaced as the
// raw config.Load error.
func loadCredentials(cmd *cobra.Command) (*config.Config, error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	usingDefault := cfgPath == ""
	if usingDefault {
		p, err := config.DefaultPath()
		if err != nil {
			return nil, err
		}
		cfgPath = p
	}
	if usingDefault {
		if _, err := os.Stat(cfgPath); errors.Is(err, fs.ErrNotExist) {
			if err := maybeMigrateChef(cmd, cfgPath); err != nil {
				return nil, err
			}
		}
	}
	return config.Load(cfgPath)
}

// maybeMigrateChef offers to migrate ~/.chef/credentials when the
// default cinc credentials file is missing. If stdin is not
// interactive, if no chef file exists, or if the user declines, it
// returns an error pointing them at `cinc configure`.
func maybeMigrateChef(cmd *cobra.Command, cincPath string) error {
	if !stdinIsTTY() {
		return missingCredentialsError(cincPath)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	chefPath := filepath.Join(home, ".chef", "credentials")
	if _, err := os.Stat(chefPath); err != nil {
		return missingCredentialsError(cincPath)
	}

	out := cmd.ErrOrStderr()
	fmt.Fprintf(out, "Found %s. Migrate to %s? [Y/n] ", chefPath, cincPath)
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "n", "no":
		return missingCredentialsError(cincPath)
	}

	n, err := migrateChef(chefPath, cincPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote %s with %d profile(s).\n", cincPath, n)
	return nil
}

func missingCredentialsError(cincPath string) error {
	return fmt.Errorf("no credentials at %s; run `cinc configure` to create one", cincPath)
}
