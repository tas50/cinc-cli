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

// openClientForm builds a huh form for the user-editable fields of
// an APIClient. APIClient.Name is the resource identifier and is
// pinned by the path argument, so the form treats it as read-only
// context rather than an editable input. The chef_key block is
// returned by the server on create and is not user-edited, so it is
// likewise omitted from the form.
func openClientForm(in *cinc.APIClient) (*cinc.APIClient, error) {
	updated := *in
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Edit client %q", in.Name)).
				Description("Validator client?").
				Affirmative("yes").
				Negative("no").
				Value(&updated.Validator),
		),
	)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("cinc: edit aborted: %w", err)
	}
	return &updated, nil
}
