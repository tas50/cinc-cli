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

// maybeMigrateChef offers chef migration when the default cinc
// credentials file is missing. If the user accepts and migration
// succeeds the function returns nil. Otherwise — no TTY, no chef
// file, declined, or write error — it returns an error pointing at
// `cinc configure`, since the caller (a server-touching command)
// cannot proceed.
func maybeMigrateChef(cmd *cobra.Command, cincPath string) error {
	migrated, err := offerChefMigration(cmd, cincPath)
	if err != nil {
		return err
	}
	if !migrated {
		return missingCredentialsError(cincPath)
	}
	return nil
}

// offerChefMigration welcomes a first-time user and, when an
// existing ~/.chef/credentials file is present, prompts them to
// migrate it to cincPath. It returns (true, nil) when a migration
// ran, (false, nil) when no migration was attempted for any benign
// reason (non-TTY, no chef file, declined), and (false, err) when
// migration was attempted but failed.
func offerChefMigration(cmd *cobra.Command, cincPath string) (bool, error) {
	if !stdinIsTTY() {
		return false, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false, nil
	}

	out := cmd.ErrOrStderr()
	fmt.Fprintln(out, "Welcome to cinc!")
	fmt.Fprintln(out)

	chefPath := filepath.Join(home, ".chef", "credentials")
	if _, err := os.Stat(chefPath); err != nil {
		return false, nil
	}

	fmt.Fprintf(out, "We found an existing Chef config at %s. Want us to migrate it to %s for you? [Y/n] ", chefPath, cincPath)
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "n", "no":
		fmt.Fprintln(out, "No problem — run `cinc configure` whenever you're ready to set up a profile.")
		return false, nil
	}

	n, err := migrateChef(chefPath, cincPath)
	if err != nil {
		return false, err
	}
	fmt.Fprintf(out, "Done! Wrote %d profile(s) to %s.\n", n, cincPath)
	return true, nil
}

func missingCredentialsError(cincPath string) error {
	return fmt.Errorf("no credentials yet at %s — run `cinc configure` to set one up", cincPath)
}
