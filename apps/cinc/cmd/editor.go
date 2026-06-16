package cmd

import (
	"bytes"
	"encoding/json"

	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/jsoneditor"
	"github.com/tas50/cinc-cli/cli/nodeedit"
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
	editNode        = openNodeEditor
	editRole        = openObjectJSONEditor[cinc.Role]
	editEnvironment = openObjectJSONEditor[cinc.Environment]
	editUser        = openObjectJSONEditor[cinc.User]
	editGroup       = openObjectJSONEditor[cinc.Group]
	editKey         = openObjectJSONEditor[cinc.Key]
)

// openNodeEditor drives the interactive node editor: a non-JSON form for
// the scalar fields (chef_environment, run_list, policy_name, policy_group)
// where each Chef attribute precedence level — normal, default, override,
// automatic — is its own row that opens the shared JSON editor. The whole
// node is carried through, so nothing outside the edited fields is lost.
func openNodeEditor(in *cinc.Node) (*cinc.Node, error) {
	return nodeedit.Run(in)
}

// nodeEditableView is the canonical editable shape of a node, with every
// section present (empty map, empty list, or null) so nil and empty compare
// alike. All four attribute precedence levels are included because each is
// editable through the node editor. chef_environment defaults to
// "_default", matching the server's implicit environment.
func nodeEditableView(n *cinc.Node) map[string]any {
	env := n.Environment
	if env == "" {
		env = "_default"
	}
	bag := func(a cinc.Attributes) map[string]any {
		if a == nil {
			return map[string]any{}
		}
		return map[string]any(a)
	}
	runList := n.RunList
	if runList == nil {
		runList = []string{}
	}
	return map[string]any{
		"name":             n.Name,
		"chef_environment": env,
		"normal":           bag(n.Normal),
		"default":          bag(n.Default),
		"override":         bag(n.Override),
		"automatic":        bag(n.Automatic),
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
