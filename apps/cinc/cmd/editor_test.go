package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

// A key added at the top level of a typed object has no home in the model
// and would be silently dropped on unmarshal, making the edit look like no
// change. strictJSON surfaces it as an error so the editor keeps the user
// editing instead of reporting the object "unchanged".
func TestStrictJSONRejectsUnknownTopLevelKey(t *testing.T) {
	validate := strictJSON[cinc.Node]()
	err := validate([]byte(`{"name":"db01","run_list":[],"newKey":null}`))
	if err == nil {
		t.Fatal("expected an unknown-field error for a new top-level key")
	}
	if !strings.Contains(err.Error(), "newKey") {
		t.Errorf("error should name the unknown field, got %v", err)
	}
}

func TestStrictJSONAcceptsKnownFieldsAndFreeFormAttributes(t *testing.T) {
	validate := strictJSON[cinc.Node]()
	// Attribute bags are maps, so arbitrary keys *inside* them are fine.
	in := `{"name":"db01","run_list":["recipe[nginx]"],"normal":{"anything":1}}`
	if err := validate([]byte(in)); err != nil {
		t.Errorf("valid node JSON should pass: %v", err)
	}
}

func TestStrictJSONRejectsMalformed(t *testing.T) {
	if err := strictJSON[cinc.Node]()([]byte(`{`)); err == nil {
		t.Error("expected an error for malformed JSON")
	}
}

// A bare node must edit with the full skeleton visible — every scalar field
// and all four attribute precedence levels — not just the fields the struct
// happens to keep after omitempty.
func TestNodeEditableViewShowsFullSkeleton(t *testing.T) {
	view := nodeEditableView(&cinc.Node{Name: "db01"})
	out, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"name":"db01"`,
		`"chef_environment":"_default"`,
		`"normal":{}`,
		`"default":{}`,
		`"override":{}`,
		`"automatic":{}`,
		`"run_list":[]`,
		`"policy_name":null`,
		`"policy_group":null`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("editable view missing %s, got %s", want, out)
		}
	}
}

func TestNodeEditUnchangedDetectsAnyBagChange(t *testing.T) {
	bare := &cinc.Node{Name: "db01"}
	for _, tc := range []struct {
		name string
		node *cinc.Node
	}{
		{"default", &cinc.Node{Name: "db01", Default: cinc.Attributes{"x": 1}}},
		{"override", &cinc.Node{Name: "db01", Override: cinc.Attributes{"x": 1}}},
		{"automatic", &cinc.Node{Name: "db01", Automatic: cinc.Attributes{"x": 1}}},
	} {
		if nodeEditUnchanged(bare, tc.node) {
			t.Errorf("a change to the %s bag should read as changed", tc.name)
		}
	}
}

func TestNodeEditUnchangedTreatsNilAndEmptyAlike(t *testing.T) {
	bare := &cinc.Node{Name: "db01"}
	skeleton := &cinc.Node{
		Name:        "db01",
		Environment: "_default",
		Normal:      cinc.Attributes{},
		RunList:     []string{},
	}
	if !nodeEditUnchanged(bare, skeleton) {
		t.Error("a re-saved untouched node should read as unchanged")
	}

	changed := &cinc.Node{Name: "db01", Normal: cinc.Attributes{"x": 1}}
	if nodeEditUnchanged(bare, changed) {
		t.Error("an added attribute should read as changed")
	}
}
