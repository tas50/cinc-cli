// Package config loads and resolves the cinc CLI configuration file: a
// TOML document holding a set of named connection profiles.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Profile describes how to connect to a single Chef/Cinc Server.
type Profile struct {
	ServerURL  string `toml:"server_url"`
	Org        string `toml:"org"`
	ClientName string `toml:"client_name"`
	KeyPath    string `toml:"key_path"`
}

// Validate reports whether the profile has every field required to open a
// server connection.
func (p Profile) Validate() error {
	switch {
	case p.ServerURL == "":
		return fmt.Errorf("config: profile is missing server_url")
	case p.Org == "":
		return fmt.Errorf("config: profile is missing org")
	case p.ClientName == "":
		return fmt.Errorf("config: profile is missing client_name")
	case p.KeyPath == "":
		return fmt.Errorf("config: profile is missing key_path")
	}
	return nil
}

// Config is the parsed contents of a cinc configuration file.
type Config struct {
	DefaultProfile string             `toml:"default_profile"`
	Profiles       map[string]Profile `toml:"profiles"`
}

// DefaultPath returns the default configuration file location,
// ~/.cinc/config.toml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: locate home directory: %w", err)
	}
	return filepath.Join(home, ".cinc", "config.toml"), nil
}

// Load reads and parses the TOML configuration file at path.
func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &cfg, nil
}

// Profile resolves a connection profile by name. An empty name selects the
// configured default profile.
func (c *Config) Profile(name string) (Profile, error) {
	if name == "" {
		name = c.DefaultProfile
	}
	if name == "" {
		return Profile{}, fmt.Errorf("config: no profile specified and no default_profile set")
	}
	p, ok := c.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("config: unknown profile %q", name)
	}
	return p, nil
}
