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
	cinc "github.com/tas50/cinc-api"
)

// Profile describes how to connect to a single Cinc Server. It is the
// resolved form of an on-disk credentials entry: the configured server URL
// has already been split into a bare server URL and an organization name.
type Profile struct {
	ServerURL       string
	Org             string
	SupermarketSite string
	ClientName      string
	KeyPath         string
	SSLVerifyMode   string

	// SecretFile is the path to the encrypted data bag secret used to
	// encrypt and decrypt `cinc databag secret` items. The on-disk key
	// is `secret_file`, matching knife's `knife[:secret_file]`, so the
	// same key serves cinc and chef users.
	SecretFile string

	// RawServerURL is the server URL exactly as written in the config, before
	// it is split into ServerURL + Org. It is preserved even when the URL is
	// malformed (ServerURL/Org are then empty) so validation can report the
	// precise problem instead of the whole load failing.
	RawServerURL string
}

// rawProfile is the on-disk shape of one credentials section. Both the
// chef-prefixed key names knife writes and their cinc-prefixed equivalents
// are accepted; when both appear in the same profile the cinc_-prefixed
// value wins.
type rawProfile struct {
	CincServerURL   string `toml:"cinc_server_url,omitempty"`
	ChefServerURL   string `toml:"chef_server_url,omitempty"`
	SupermarketSite string `toml:"supermarket_site,omitempty"`
	ClientName      string `toml:"client_name,omitempty"`
	ClientKey       string `toml:"client_key,omitempty"`
	SSLVerifyMode   string `toml:"ssl_verify_mode,omitempty"`
	SecretFile      string `toml:"secret_file,omitempty"`
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
	if err := p.ValidateIdentity(); err != nil {
		return err
	}
	switch {
	case p.ServerURL == "" && p.RawServerURL != "":
		// A server URL was written but didn't parse; report exactly why.
		if _, _, err := cinc.ParseServerURL(p.RawServerURL); err != nil {
			return fmt.Errorf("config: %w", err)
		}
		return fmt.Errorf("config: profile is missing cinc_server_url (or chef_server_url)")
	case p.ServerURL == "":
		return fmt.Errorf("config: profile is missing cinc_server_url (or chef_server_url)")
	case p.Org == "":
		return fmt.Errorf("config: profile is missing the /organizations/<org> segment in its server URL")
	}
	return nil
}

// ValidateIdentity reports whether the profile has the fields needed to sign
// requests that do not target a Cinc Server organization, such as
// Supermarket uploads.
func (p Profile) ValidateIdentity() error {
	switch {
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

// ValidationIssue describes one profile-level configuration problem.
type ValidationIssue struct {
	Profile string `json:"profile"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validate reports configuration problems across every loaded profile. It is
// intentionally local-only: it validates TOML shape and URL/profile fields but
// does not perform network calls or authenticate.
func (c *Config) Validate() []ValidationIssue {
	if len(c.Profiles) == 0 {
		return []ValidationIssue{{
			Field:   "profiles",
			Message: "config has no profiles",
		}}
	}
	var issues []ValidationIssue
	for name, profile := range c.Profiles {
		issues = append(issues, validateProfile(name, profile)...)
	}
	return issues
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
	if err := p.ValidateIdentity(); err != nil {
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
		ChefServerURL:   profileServerURL(p),
		SupermarketSite: p.SupermarketSite,
		ClientName:      p.ClientName,
		ClientKey:       p.KeyPath,
		SSLVerifyMode:   p.SSLVerifyMode,
		SecretFile:      p.SecretFile,
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
		if profile.SupermarketSite != "" {
			values["supermarket_site"] = profile.SupermarketSite
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
		if profile.SecretFile != "" {
			values["secret_file"] = profile.SecretFile
		}
		out[name] = values
	}
	return out
}

// NewProfile builds a Profile from user-supplied configure values.
func NewProfile(serverURL, clientName, clientKey, sslVerifyMode, supermarketSite string) (Profile, error) {
	var server, org string
	if serverURL != "" {
		parsedServer, parsedOrg, err := cinc.ParseServerURL(serverURL)
		if err == nil {
			server = parsedServer
			org = parsedOrg
		} else {
			if supermarketSite != "" {
				return Profile{}, err
			}
			if err := ValidateSiteURL(serverURL); err != nil {
				return Profile{}, err
			}
			supermarketSite = serverURL
		}
	}
	p := Profile{
		ServerURL:       server,
		Org:             org,
		SupermarketSite: supermarketSite,
		ClientName:      clientName,
		KeyPath:         clientKey,
		SSLVerifyMode:   sslVerifyMode,
	}
	if err := p.ValidateIdentity(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func profileServerURL(p Profile) string {
	if p.ServerURL == "" || p.Org == "" {
		return ""
	}
	return strings.TrimRight(p.ServerURL, "/") + "/organizations/" + p.Org
}

// resolveProfile turns a raw on-disk entry into a usable Profile, splitting
// the configured server URL into a server URL and an organization name.
func resolveProfile(rp rawProfile) (Profile, error) {
	p := Profile{
		SupermarketSite: rp.SupermarketSite,
		ClientName:      rp.ClientName,
		KeyPath:         rp.ClientKey,
		SSLVerifyMode:   rp.SSLVerifyMode,
		SecretFile:      rp.SecretFile,
	}
	if raw := rp.serverURL(); raw != "" {
		// Preserve the raw URL even when it doesn't parse, so validation can
		// report the precise problem instead of the whole load failing. A
		// malformed URL leaves ServerURL/Org empty; Profile.Validate surfaces
		// the parse error for callers that need a usable connection.
		p.RawServerURL = raw
		if server, org, err := cinc.ParseServerURL(raw); err == nil {
			p.ServerURL = server
			p.Org = org
		}
	}
	return p, nil
}

func ValidateSiteURL(raw string) error {
	u, parseErr := url.Parse(raw)
	if parseErr != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid site URL %q", raw)
	}
	return nil
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

func validateProfile(name string, p Profile) []ValidationIssue {
	var issues []ValidationIssue
	add := func(field, message string) {
		issues = append(issues, ValidationIssue{Profile: name, Field: field, Message: message})
	}
	if p.ClientName == "" {
		add("client_name", "client_name is required")
	}
	if p.KeyPath == "" {
		add("client_key", "client_key is required")
	}
	switch {
	case p.ServerURL == "" && p.Org == "" && p.RawServerURL == "" && p.SupermarketSite == "":
		add("endpoint", "configure cinc_server_url, chef_server_url, or supermarket_site")
	case p.RawServerURL != "" && p.ServerURL == "":
		// A server URL was set but couldn't be parsed into server + org.
		add("cinc_server_url", "server URL must include /organizations/<org>")
	case (p.ServerURL == "") != (p.Org == ""):
		add("cinc_server_url", "server URL must include /organizations/<org>")
	}
	if p.SupermarketSite != "" {
		if err := ValidateSiteURL(p.SupermarketSite); err != nil {
			add("supermarket_site", err.Error())
		}
	}
	if p.SSLVerifyMode != "" && p.SSLVerifyMode != ":verify_peer" && p.SSLVerifyMode != ":verify_none" {
		add("ssl_verify_mode", "ssl_verify_mode must be :verify_peer or :verify_none")
	}
	return issues
}
