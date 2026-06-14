package cmd

import (
	"encoding/json"

	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/jsoneditor"
)

// editClient presents the supplied client as pretty-printed JSON in the
// shared JSON editor and returns the parsed result. It is a package
// variable so tests can override it without spawning a TUI.
var editClient = openClientJSONEditor

func openClientJSONEditor(in *cinc.APIClient) (*cinc.APIClient, error) {
	initial, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, err
	}
	edited, err := jsoneditor.Run(initial, func(b []byte) error {
		var c cinc.APIClient
		return json.Unmarshal(b, &c)
	})
	if err != nil {
		return nil, err
	}
	var out cinc.APIClient
	if err := json.Unmarshal(edited, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// editRole, editEnvironment, editUser, and editGroup present the supplied
// object as pretty-printed JSON in the shared editor and return the parsed
// result. Each is a package variable so tests can override it without
// spawning a TUI, mirroring editClient.
var (
	editNode        = openObjectJSONEditor[cinc.Node]
	editRole        = openObjectJSONEditor[cinc.Role]
	editEnvironment = openObjectJSONEditor[cinc.Environment]
	editUser        = openObjectJSONEditor[cinc.User]
	editGroup       = openObjectJSONEditor[cinc.Group]
	editKey         = openObjectJSONEditor[cinc.Key]
)

// openObjectJSONEditor is the generic edit-as-JSON flow shared by the simple
// server-object nouns: marshal in, edit with round-trip validation, unmarshal
// out.
func openObjectJSONEditor[T any](in *T) (*T, error) {
	initial, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, err
	}
	edited, err := jsoneditor.Run(initial, func(b []byte) error {
		var v T
		return json.Unmarshal(b, &v)
	})
	if err != nil {
		return nil, err
	}
	var out T
	if err := json.Unmarshal(edited, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// editDataBagItem opens a data bag item in the shared JSON editor.
// Validation rejects malformed JSON and items missing an "id" key. It
// is a package variable so tests can stub it.
var editDataBagItem = openDataBagItemJSONEditor

func openDataBagItemJSONEditor(in cinc.DataBagItem) (cinc.DataBagItem, error) {
	initial, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, err
	}
	edited, err := jsoneditor.Run(initial, validateDataBagItem)
	if err != nil {
		return nil, err
	}
	var out cinc.DataBagItem
	if err := json.Unmarshal(edited, &out); err != nil {
		return nil, err
	}
	return out, nil
}
