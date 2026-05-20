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
	CincServerURL string `toml:"cinc_server_url,omitempty"`
	ChefServerURL string `toml:"chef_server_url,omitempty"`
	ClientName    string `toml:"client_name,omitempty"`
	ClientKey     string `toml:"client_key,omitempty"`
	SSLVerifyMode string `toml:"ssl_verify_mode,omitempty"`
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

// WriteProfile creates or updates one profile in the credentials file at path.
// Existing profiles are preserved, but comments and original key ordering are
// not retained because the file is rewritten as TOML.
func WriteProfile(path, name string, p Profile) error {
	if name == "" {
		return fmt.Errorf("config: profile name is required")
	}
	if err := p.Validate(); err != nil {
		return err
	}
	raw := map[string]rawProfile{}
	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, &raw); err != nil {
			return fmt.Errorf("config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("config: stat %s: %w", path, err)
	}
	raw[name] = rawProfile{
		ChefServerURL: profileServerURL(p),
		ClientName:    p.ClientName,
		ClientKey:     p.KeyPath,
		SSLVerifyMode: p.SSLVerifyMode,
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("config: create credentials directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	if err := toml.NewEncoder(f).Encode(tomlProfiles(raw)); err != nil {
		_ = f.Close()
		return fmt.Errorf("config: encode %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("config: close %s: %w", path, err)
	}
	return nil
}

func tomlProfiles(raw map[string]rawProfile) map[string]map[string]string {
	out := make(map[string]map[string]string, len(raw))
	for name, profile := range raw {
		values := map[string]string{}
		if profile.CincServerURL != "" {
			values["cinc_server_url"] = profile.CincServerURL
		}
		if profile.ChefServerURL != "" {
			values["chef_server_url"] = profile.ChefServerURL
		}
		if profile.ClientName != "" {
			values["client_name"] = profile.ClientName
		}
		if profile.ClientKey != "" {
			values["client_key"] = profile.ClientKey
		}
		if profile.SSLVerifyMode != "" {
			values["ssl_verify_mode"] = profile.SSLVerifyMode
		}
		out[name] = values
	}
	return out
}

// NewProfile builds a Profile from user-supplied configure values.
func NewProfile(serverURL, clientName, clientKey, sslVerifyMode string) (Profile, error) {
	server, org, err := splitServerURL(serverURL)
	if err != nil {
		return Profile{}, err
	}
	p := Profile{
		ServerURL:     server,
		Org:           org,
		ClientName:    clientName,
		KeyPath:       clientKey,
		SSLVerifyMode: sslVerifyMode,
	}
	if err := p.Validate(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func profileServerURL(p Profile) string {
	return strings.TrimRight(p.ServerURL, "/") + "/organizations/" + p.Org
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
