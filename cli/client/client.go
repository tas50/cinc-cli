// Package client builds a configured cinc-api client from CLI
// configuration. It is the single seam between cinc CLI state and the
// server API library.
package client

import (
	"fmt"

	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/config"
)

// New builds a cinc-api client from a resolved configuration profile. All
// server authentication and transport is handled by the cinc-api library;
// this function only loads the signing key and wires up the connection
// parameters.
func New(p config.Profile) (*cinc.Client, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	key, err := cinc.LoadKeyFile(p.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}

	c, err := cinc.NewClient(cinc.Config{
		ServerURL:  p.ServerURL,
		Org:        p.Org,
		ClientName: p.ClientName,
		Key:        key,
	})
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	return c, nil
}
