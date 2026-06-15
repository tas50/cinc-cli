package cmd

import (
	"bytes"
	"encoding/json"

	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/jsoneditor"
)

// strictJSON returns a save-validation func for the typed editors. It
// rejects malformed JSON and any key the model T does not recognize, so a
// key added in the editor surfaces as an "unknown field" error rather than
// being silently dropped on unmarshal — which would make a real edit look
// like no change at all and report the object "unchanged".
func strictJSON[T any]() func([]byte) error {
	return func(b []byte) error {
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		var v T
		return dec.Decode(&v)
	}
}

// editClient presents the supplied client as pretty-printed JSON in the
// shared JSON editor and returns the parsed result. It is a package
// variable so tests can override it without spawning a TUI.
var editClient = openClientJSONEditor

func openClientJSONEditor(in *cinc.APIClient) (*cinc.APIClient, error) {
	initial, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, err
	}
	edited, err := jsoneditor.Run(initial, strictJSON[cinc.APIClient]())
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
	editNode        = openNodeJSONEditor
	editRole        = openObjectJSONEditor[cinc.Role]
	editEnvironment = openObjectJSONEditor[cinc.Environment]
	editUser        = openObjectJSONEditor[cinc.User]
	editGroup       = openObjectJSONEditor[cinc.Group]
	editKey         = openObjectJSONEditor[cinc.Key]
)

// openNodeJSONEditor edits a node the way `knife node edit` does: it shows
// the full editable skeleton — name, chef_environment, normal, run_list,
// policy_name, policy_group — even when a section is empty, so nothing is
// hidden behind a struct's omitempty. The large computed attributes
// (default, override, automatic) are kept out of the editor but preserved
// untouched on save.
func openNodeJSONEditor(in *cinc.Node) (*cinc.Node, error) {
	seed, err := json.MarshalIndent(nodeEditableView(in), "", "  ")
	if err != nil {
		return nil, err
	}
	edited, err := jsoneditor.Run(seed, strictJSON[cinc.Node]())
	if err != nil {
		return nil, err
	}
	var ev cinc.Node
	if err := json.Unmarshal(edited, &ev); err != nil {
		return nil, err
	}
	merged := applyNodeEdit(in, &ev)
	return &merged, nil
}

// nodeEditableView is the knife-style editable subset of a node, with every
// section present (empty map, empty list, or null) so the editor always
// shows the full shape. chef_environment defaults to "_default", matching
// the server's implicit environment.
func nodeEditableView(n *cinc.Node) map[string]any {
	env := n.Environment
	if env == "" {
		env = "_default"
	}
	normal := map[string]any(n.Normal)
	if normal == nil {
		normal = map[string]any{}
	}
	runList := n.RunList
	if runList == nil {
		runList = []string{}
	}
	return map[string]any{
		"name":             n.Name,
		"chef_environment": env,
		"normal":           normal,
		"run_list":         runList,
		"policy_name":      nullableString(n.PolicyName),
		"policy_group":     nullableString(n.PolicyGroup),
	}
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// applyNodeEdit folds the edited editable fields back onto the original
// node, preserving its computed default/override/automatic attributes.
func applyNodeEdit(orig, edited *cinc.Node) cinc.Node {
	merged := *orig
	merged.Name = edited.Name
	merged.Environment = edited.Environment
	merged.RunList = edited.RunList
	merged.Normal = edited.Normal
	merged.PolicyName = edited.PolicyName
	merged.PolicyGroup = edited.PolicyGroup
	return merged
}

// nodeEditUnchanged reports whether two nodes have the same editable view,
// so a re-save of an untouched node (where nil and {} are equivalent)
// reports "unchanged" rather than a spurious update.
func nodeEditUnchanged(a, b *cinc.Node) bool {
	av, _ := json.Marshal(nodeEditableView(a))
	bv, _ := json.Marshal(nodeEditableView(b))
	return bytes.Equal(av, bv)
}

// openObjectJSONEditor is the generic edit-as-JSON flow shared by the simple
// server-object nouns: marshal in, edit with round-trip validation, unmarshal
// out.
func openObjectJSONEditor[T any](in *T) (*T, error) {
	initial, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, err
	}
	edited, err := jsoneditor.Run(initial, strictJSON[T]())
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
