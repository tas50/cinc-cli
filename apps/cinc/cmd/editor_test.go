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

// A bare node must edit with the full knife-style skeleton visible, not
// just the fields the struct happens to keep after omitempty.
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
		`"run_list":[]`,
		`"policy_name":null`,
		`"policy_group":null`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("editable view missing %s, got %s", want, out)
		}
	}
}

func TestApplyNodeEditPreservesComputedAttributes(t *testing.T) {
	orig := &cinc.Node{
		Name:      "db01",
		Automatic: cinc.Attributes{"platform": "ubuntu"},
		Default:   cinc.Attributes{"role_attr": 1},
	}
	edited := &cinc.Node{Name: "db01", Normal: cinc.Attributes{"role": "db"}}
	merged := applyNodeEdit(orig, edited)

	if merged.Normal["role"] != "db" {
		t.Errorf("edited normal not applied: %+v", merged.Normal)
	}
	if merged.Automatic["platform"] != "ubuntu" {
		t.Errorf("automatic attributes not preserved: %+v", merged.Automatic)
	}
	if merged.Default["role_attr"] != 1 {
		t.Errorf("default attributes not preserved: %+v", merged.Default)
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
