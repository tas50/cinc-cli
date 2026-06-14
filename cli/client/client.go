// Package client builds a configured cinc-api client from CLI
// configuration. It is the single seam between cinc CLI state and the
// server API library.
package client

import (
	"fmt"
	"io"
	"os"

	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/config"
)

// tlsWarnWriter is where the "TLS verification disabled" warning is written. It
// is a package variable so tests can capture it.
var tlsWarnWriter io.Writer = os.Stderr

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

	var opts []cinc.Option
	if p.SSLVerifyMode == ":verify_none" {
		opts = append(opts, cinc.WithSkipTLSVerify(true))
		fmt.Fprintf(tlsWarnWriter,
			"Warning: TLS certificate verification is disabled (ssl_verify_mode = :verify_none); the connection to %s is not authenticated and is vulnerable to interception.\n",
			p.ServerURL)
	}

	c, err := cinc.NewClient(cinc.Config{
		ServerURL:  p.ServerURL,
		Org:        p.Org,
		ClientName: p.ClientName,
		Key:        key,
	}, opts...)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	return c, nil
}
