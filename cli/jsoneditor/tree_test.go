package jsoneditor

import (
	"strings"
	"testing"
)

func TestParseTreeRoundTripPreservesOrder(t *testing.T) {
	// Members must come back in document order, not alphabetized.
	in := `{"b":2,"a":1,"nested":{"z":true,"y":null}}`
	root, err := parseTree([]byte(in))
	if err != nil {
		t.Fatalf("parseTree: %v", err)
	}
	got := string(root.bytes())
	want := `{
  "b": 2,
  "a": 1,
  "nested": {
    "z": true,
    "y": null
  }
}`
	if got != want {
		t.Errorf("serialize mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestParseTreeArraysAndScalars(t *testing.T) {
	in := `{"run_list":["recipe[nginx]","recipe[base]"],"tags":{},"empty":[]}`
	root, err := parseTree([]byte(in))
	if err != nil {
		t.Fatalf("parseTree: %v", err)
	}
	got := string(root.bytes())
	want := `{
  "run_list": [
    "recipe[nginx]",
    "recipe[base]"
  ],
  "tags": {},
  "empty": []
}`
	if got != want {
		t.Errorf("serialize mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestParseTreeStringsAreEscaped(t *testing.T) {
	root, err := parseTree([]byte(`{"k":"a\"b\tc"}`))
	if err != nil {
		t.Fatalf("parseTree: %v", err)
	}
	got := string(root.bytes())
	if !strings.Contains(got, `"a\"b\tc"`) {
		t.Errorf("string not re-escaped canonically, got %q", got)
	}
}

func TestParseTreeRejectsInvalidJSON(t *testing.T) {
	if _, err := parseTree([]byte(`{"k":}`)); err == nil {
		t.Error("expected an error for malformed JSON")
	}
	if _, err := parseTree([]byte(`{} trailing`)); err == nil {
		t.Error("expected an error for trailing data")
	}
}

func TestParseTreeNumbersKeepFormatting(t *testing.T) {
	root, err := parseTree([]byte(`[1,2.5,-3,1000000]`))
	if err != nil {
		t.Fatalf("parseTree: %v", err)
	}
	got := string(root.bytes())
	want := `[
  1,
  2.5,
  -3,
  1000000
]`
	if got != want {
		t.Errorf("number formatting changed:\n got: %q\nwant: %q", got, want)
	}
}
