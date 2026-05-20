// Package config loads and resolves the cinc CLI credentials file: a
// TOML document holding a set of named connection profiles, in the same
// shape as Chef's `~/.chef/credentials` file.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Profile describes how to connect to a single Cinc/Chef Server. It is the
// resolved form of an on-disk credentials entry: the configured server URL
// has already been split into a bare server URL and an organization name.
type Profile struct {
	ServerURL     string
	Org           string
	ClientName    string
	KeyPath       string
	SSLVerifyMode string
}

// rawProfile is the on-disk shape of one credentials section. Both the
// chef-prefixed key names knife writes and their cinc-prefixed equivalents
// are accepted; when both appear in the same profile the cinc_-prefixed
// value wins.
type rawProfile struct {
	CincServerURL string `toml:"cinc_server_url"`
	ChefServerURL string `toml:"chef_server_url"`
	ClientName    string `toml:"client_name"`
	ClientKey     string `toml:"client_key"`
	SSLVerifyMode string `toml:"ssl_verify_mode"`
}

// serverURL returns the configured server URL, preferring the
// cinc-prefixed key over the chef-prefixed key when both are set.
func (rp rawProfile) serverURL() string {
	if rp.CincServerURL != "" {
		return rp.CincServerURL
	}
	return rp.ChefServerURL
}

// Validate reports whether the profile has every field required to open a
// server connection.
func (p Profile) Validate() error {
	switch {
	case p.ServerURL == "":
		return fmt.Errorf("config: profile is missing cinc_server_url (or chef_server_url)")
	case p.Org == "":
		return fmt.Errorf("config: profile is missing the /organizations/<org> segment in its server URL")
	case p.ClientName == "":
		return fmt.Errorf("config: profile is missing client_name")
	case p.KeyPath == "":
		return fmt.Errorf("config: profile is missing client_key")
	}
	return nil
}

// Config is the parsed contents of a cinc credentials file.
type Config struct {
	Profiles map[string]Profile
}

// DefaultPath returns the default credentials file location,
// ~/.cinc/credentials.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: locate home directory: %w", err)
	}
	return filepath.Join(home, ".cinc", "credentials"), nil
}

// Load reads and parses the credentials file at path. Each top-level TOML
// section becomes a named profile.
func Load(path string) (*Config, error) {
	var raw map[string]rawProfile
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	cfg := &Config{Profiles: make(map[string]Profile, len(raw))}
	for name, rp := range raw {
		p, err := resolveProfile(rp)
		if err != nil {
			return nil, fmt.Errorf("config: profile %q: %w", name, err)
		}
		cfg.Profiles[name] = p
	}
	return cfg, nil
}

// resolveProfile turns a raw on-disk entry into a usable Profile, splitting
// the configured server URL into a server URL and an organization name.
func resolveProfile(rp rawProfile) (Profile, error) {
	p := Profile{
		ClientName:    rp.ClientName,
		KeyPath:       rp.ClientKey,
		SSLVerifyMode: rp.SSLVerifyMode,
	}
	if raw := rp.serverURL(); raw != "" {
		server, org, err := splitServerURL(raw)
		if err != nil {
			return Profile{}, err
		}
		p.ServerURL = server
		p.Org = org
	}
	return p, nil
}

// splitServerURL parses a server URL of the form
// `https://host[:port]/organizations/<org>` into its base server URL
// (`https://host[:port]`) and the organization name.
func splitServerURL(raw string) (server, org string, err error) {
	u, parseErr := url.Parse(raw)
	if parseErr != nil || u.Scheme == "" || u.Host == "" {
		return "", "", fmt.Errorf("invalid server URL %q", raw)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "organizations" || parts[1] == "" {
		return "", "", fmt.Errorf("server URL %q must end with /organizations/<org>", raw)
	}
	return u.Scheme + "://" + u.Host, parts[1], nil
}

// Profile resolves a connection profile by name. An empty name selects
// the profile named by $CINC_PROFILE, then $CHEF_PROFILE, falling back to
// "default". When both env vars are set CINC_PROFILE wins.
func (c *Config) Profile(name string) (Profile, error) {
	if name == "" {
		name = envProfile()
	}
	if name == "" {
		name = "default"
	}
	p, ok := c.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("config: unknown profile %q", name)
	}
	return p, nil
}

// envProfile returns the profile name selected by environment variables,
// preferring CINC_PROFILE over CHEF_PROFILE.
func envProfile() string {
	if v := os.Getenv("CINC_PROFILE"); v != "" {
		return v
	}
	return os.Getenv("CHEF_PROFILE")
}
