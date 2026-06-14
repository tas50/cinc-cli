package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/client"
	"github.com/tas50/cinc-cli/cli/config"
	"github.com/tas50/cinc-cli/cli/printer"
	"github.com/tas50/cinc-cli/cli/setup"
	"github.com/tas50/cinc-cli/cli/supermarket"
)

// errFirstRunCompleted is a sentinel returned by loadCredentials after
// the first-run flow has written the credentials file. It signals the
// caller (and ultimately Execute) that the user has been welcomed and
// the invocation should exit cleanly without running the original
// server-touching command, so a fresh user isn't surprised by their
// `cinc node list` running against a server they just typed in.
var errFirstRunCompleted = errors.New("cinc: first-run setup completed")

// errAlreadyReported marks an error whose details a command has already
// printed for the user (e.g. `config validate`'s per-issue list). Execute
// returns it for a non-zero exit but does not print a second generic
// "Error: ..." line.
var errAlreadyReported = errors.New("cinc: already reported")

// migrateChef is the function used to migrate ~/.chef/credentials when the
// default cinc credentials file is missing. It is a package-level variable
// so tests can swap in a fake.
var migrateChef = setup.MigrateChef

// runFirstRunConfigure interactively configures a fresh credentials
// profile, the same way `cinc config create` does. It is a package-level
// variable so tests can swap in a fake.
var runFirstRunConfigure = realRunFirstRunConfigure

// stdinIsTTY reports whether os.Stdin is connected to an interactive
// terminal. It uses the TCGETS ioctl via go-isatty so the answer is
// the same whatever opaque file type the shell hands us — a regular
// pty, a tmux/screen-allocated pty, or a Cygwin-style mintty terminal.
// Tests swap this var directly.
var stdinIsTTY = func() bool {
	fd := os.Stdin.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
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
	c, err := client.New(profile)
	if err != nil {
		return nil, friendlyKeyFileError(cmd, profile, err)
	}
	return c, nil
}

// friendlyKeyFileError rewrites a "client key file missing/unreadable"
// error into a conversational message that names both the key path
// and the credentials file where client_key is configured. Other
// errors pass through unchanged.
func friendlyKeyFileError(cmd *cobra.Command, p config.Profile, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("can't find your client key at %s — it's set as client_key in %s. Create the key file, or update client_key to point at the right path.", p.KeyPath, resolveConfigPath(cmd))
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("can't read your client key at %s — it's set as client_key in %s. Check the file's permissions; cinc needs to read it to sign requests.", p.KeyPath, resolveConfigPath(cmd))
	}
	return err
}

// resolveConfigPath returns the credentials file path the command is
// pointed at, matching loadCredentials' behavior so error messages
// name the same path the loader used.
func resolveConfigPath(cmd *cobra.Command) string {
	if p, _ := cmd.Flags().GetString("config"); p != "" {
		return p
	}
	if p, err := config.DefaultPath(); err == nil {
		return p
	}
	return ""
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
// path) and the file is missing, the first-run flow runs before the
// load is retried. An explicit --config pointing at a missing file is
// surfaced as the raw config.Load error.
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
			if err := maybeFirstRun(cmd, cfgPath); err != nil {
				return nil, err
			}
			return nil, errFirstRunCompleted
		}
	}
	return config.Load(cfgPath)
}

// maybeFirstRun runs the welcome flow when the default cinc
// credentials file is missing. If credentials were set up (migrated
// or configured), it returns nil. Otherwise — no TTY, declined
// migration, or a write error — it returns an error pointing the
// caller (a server-touching command) at `cinc config create`.
func maybeFirstRun(cmd *cobra.Command, cincPath string) error {
	succeeded, err := offerFirstRun(cmd, cincPath)
	if err != nil {
		return err
	}
	if !succeeded {
		return missingCredentialsError(cincPath)
	}
	return nil
}

// offerFirstRun welcomes a first-time user and either migrates an
// existing ~/.chef/credentials file or walks them through the
// configure prompts inline. Returns (true, nil) when credentials
// were written, (false, nil) for benign no-ops (non-TTY, declined
// migration), and (false, err) on failure.
func offerFirstRun(cmd *cobra.Command, cincPath string) (bool, error) {
	if !stdinIsTTY() {
		return false, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false, nil
	}

	out := cmd.ErrOrStderr()
	fmt.Fprintln(out, "Welcome to the Cinc CLI!")
	fmt.Fprintln(out)

	chefPath := filepath.Join(home, ".chef", "credentials")
	if _, err := os.Stat(chefPath); err == nil {
		return runMigrationPrompt(cmd, chefPath, cincPath, out)
	}
	return runConfigurePrompt(cmd, cincPath, out)
}

// runMigrationPrompt asks the user whether to migrate ~/.chef/credentials
// and either runs the migration or returns a friendly decline.
func runMigrationPrompt(cmd *cobra.Command, chefPath, cincPath string, out io.Writer) (bool, error) {
	fmt.Fprintf(out, "We found an existing Chef config at %s. Want us to migrate it to %s for you? [Y/n] ", chefPath, cincPath)
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "n", "no":
		fmt.Fprintln(out, "No problem — run `cinc config create` whenever you're ready to set up a profile.")
		fmt.Fprintln(out)
		return false, nil
	}

	n, err := migrateChef(chefPath, cincPath)
	if err != nil {
		return false, err
	}
	fmt.Fprintf(out, "Done! Wrote %d profile(s) to %s.\n", n, cincPath)
	fmt.Fprintln(out)
	return true, nil
}

// runConfigurePrompt drives the interactive configure flow when no
// chef credentials file exists to migrate. The welcome line above is
// enough context to explain why the prompts are appearing; the
// configure flow prints its own opener so we don't repeat ourselves
// here.
func runConfigurePrompt(cmd *cobra.Command, cincPath string, out io.Writer) (bool, error) {
	if err := runFirstRunConfigure(cmd, cincPath); err != nil {
		return false, err
	}
	fmt.Fprintln(out)
	return true, nil
}

// realRunFirstRunConfigure delegates to the same prompt + write
// machinery `cinc config create` uses, so the on-disk result matches.
func realRunFirstRunConfigure(cmd *cobra.Command, cincPath string) error {
	answers, err := promptConfigure(cmd, configureDefaults{
		ConfigPath:      cincPath,
		ProfileName:     "default",
		SupermarketSite: supermarket.DefaultSite,
		ClientName:      defaultClientName(),
	})
	if err != nil {
		return err
	}
	if answers.ClientKey == "" {
		answers.ClientKey = defaultClientKey(answers.ClientName)
	}
	profile, err := config.NewProfile(
		answers.ChefServerURL,
		answers.ClientName,
		answers.ClientKey,
		answers.SSLVerifyMode,
		answers.SupermarketSite,
	)
	if err != nil {
		return err
	}
	if err := config.WriteProfile(answers.ConfigPath, answers.ProfileName, profile); err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Wrote credentials profile %q to %s\n", answers.ProfileName, answers.ConfigPath)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Cinc CLI is now configured and you're ready to go!")
	return nil
}

func missingCredentialsError(cincPath string) error {
	return fmt.Errorf("no credentials yet at %s — run `cinc config create` to set one up", cincPath)
}
