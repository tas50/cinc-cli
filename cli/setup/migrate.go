// Package setup handles first-run credential migration for cinc. It
// detects an existing ~/.chef/credentials file and writes the
// equivalent ~/.cinc/credentials file by delegating to cli/config.
// Guided setup of a fresh profile is handled separately by the
// `cinc config configure` command.
package setup

import (
	"fmt"

	"github.com/BurntSushi/toml"

	"github.com/tas50/cinc-cli/cli/config"
)

// chefRawProfile is the on-disk shape of one credentials section in
// either ~/.chef/credentials or ~/.cinc/credentials. Cinc- and
// chef-prefixed server URLs are both accepted; when both are set the
// cinc-prefixed value wins, matching cli/config's precedence rule.
type chefRawProfile struct {
	CincServerURL   string `toml:"cinc_server_url"`
	ChefServerURL   string `toml:"chef_server_url"`
	SupermarketSite string `toml:"supermarket_site"`
	ClientName      string `toml:"client_name"`
	ClientKey       string `toml:"client_key"`
	SSLVerifyMode   string `toml:"ssl_verify_mode"`
}

// MigrateChef reads chefPath and writes the equivalent credentials
// file to cincPath. Each profile is built via config.NewProfile (which
// validates client name, key path, and either a Chef server URL or a
// Supermarket site) and then persisted via config.WriteProfile, so the
// resulting file is byte-identical to what `cinc config configure` would
// produce for the same inputs. Returns the number of profiles
// migrated.
func MigrateChef(chefPath, cincPath string) (int, error) {
	var raw map[string]chefRawProfile
	if _, err := toml.DecodeFile(chefPath, &raw); err != nil {
		return 0, fmt.Errorf("setup: parse %s: %w", chefPath, err)
	}
	for name, rp := range raw {
		serverURL := rp.CincServerURL
		if serverURL == "" {
			serverURL = rp.ChefServerURL
		}
		profile, err := config.NewProfile(serverURL, rp.ClientName, rp.ClientKey, rp.SSLVerifyMode, rp.SupermarketSite)
		if err != nil {
			return 0, fmt.Errorf("setup: profile %q: %w", name, err)
		}
		if err := config.WriteProfile(cincPath, name, profile); err != nil {
			return 0, err
		}
	}
	return len(raw), nil
}
