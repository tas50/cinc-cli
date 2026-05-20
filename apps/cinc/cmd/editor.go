package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	cinc "github.com/tas50/cinc-api"
)

// editClient presents a field-based form for the editable parts of
// an APIClient and returns the modified copy. It is a package
// variable so tests can override it with a deterministic stub
// instead of spinning up a TUI program.
//
// cinc ships its own editor rather than shelling out to $VISUAL /
// $EDITOR so the experience is consistent across systems and works
// in minimal environments without an editor on PATH.
var editClient = openClientForm

// openClientForm builds a huh form covering every user-editable
// field on an APIClient: the validator flag and the chef_key block
// (key entry name, PEM public key, expiration date). Name is shown
// as a read-only header because cinc-api derives the PUT URL from
// APIClient.Name, so renaming via this command is not currently
// expressible. PrivateKey is never sent on a PUT (the server returns
// it on create only), so it is omitted as well.
func openClientForm(in *cinc.APIClient) (*cinc.APIClient, error) {
	updated := *in
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Editing client").
				Description(in.Name),
			huh.NewConfirm().
				Title("Validator client?").
				Affirmative("yes").
				Negative("no").
				Value(&updated.Validator),
			huh.NewInput().
				Title("Chef key name").
				Description("Identifier for the key entry (typically \"default\")").
				Value(&updated.ChefKey.Name),
			huh.NewText().
				Title("Public key (PEM)").
				Description("Replace to rotate the key; leave as-is to keep the existing one").
				Value(&updated.ChefKey.PublicKey),
			huh.NewInput().
				Title("Expiration date").
				Description("ISO 8601 timestamp, or \"infinity\"").
				Value(&updated.ChefKey.ExpiresAt),
		),
	)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("cinc: edit aborted: %w", err)
	}
	return &updated, nil
}
