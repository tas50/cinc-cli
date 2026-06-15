package jsoneditor

import (
	"strings"
	"testing"
)

// mark wraps highlighted text in guillemets so tests can assert on the
// selection without dealing with ANSI escape codes.
func mark(s string) string { return "«" + s + "»" }

func identity(s string) string { return s }

// markTheme leaves every token un-colored and wraps only the selected
// unit, so selection assertions are unaffected by syntax styling.
func markTheme() renderTheme {
	return renderTheme{
		key: identity, str: identity, num: identity,
		lit: identity, punct: identity, sel: mark,
	}
}

func renderWith(t *testing.T, in string, u unit) string {
	t.Helper()
	root, err := parseTree([]byte(in))
	if err != nil {
		t.Fatalf("parseTree: %v", err)
	}
	return render(root, u, markTheme())
}

func TestRenderHighlightsSelectedKey(t *testing.T) {
	out := renderWith(t, `{"name":"web01","port":8080}`, unit{typ: uKey, path: []int{0}})
	if !strings.Contains(out, `«"name"»: "web01"`) {
		t.Errorf("key not highlighted as a whole, got:\n%s", out)
	}
	if strings.Contains(out, `«"web01"»`) {
		t.Errorf("value should not be highlighted when key is selected, got:\n%s", out)
	}
}

func TestRenderHighlightsSelectedScalarValue(t *testing.T) {
	out := renderWith(t, `{"name":"web01","port":8080}`, unit{typ: uScalar, path: []int{0}})
	if !strings.Contains(out, `"name": «"web01"»`) {
		t.Errorf("scalar value not highlighted, got:\n%s", out)
	}
}

func TestRenderHighlightsWholeBlock(t *testing.T) {
	out := renderWith(t, `{"run_list":["a","b"]}`, unit{typ: uBlock, path: []int{0}})
	// The entire array, opening bracket through closing bracket, is wrapped.
	want := "«[\n    \"a\",\n    \"b\"\n  ]»"
	if !strings.Contains(out, want) {
		t.Errorf("block not highlighted as a whole, got:\n%s\nwant substring:\n%s", out, want)
	}
}

func TestRenderHighlightsNestedScalar(t *testing.T) {
	out := renderWith(t, `{"run_list":["a","b"]}`, unit{typ: uScalar, path: []int{0, 1}})
	if !strings.Contains(out, `«"b"»`) {
		t.Errorf("nested element not highlighted, got:\n%s", out)
	}
	if strings.Contains(out, `«"a"»`) {
		t.Errorf("only the selected element should be highlighted, got:\n%s", out)
	}
}

func TestRenderUnselectedIsPlainJSON(t *testing.T) {
	root, _ := parseTree([]byte(`{"a":1}`))
	// A unit that matches nothing leaves the document un-highlighted and,
	// under an identity theme, identical to the canonical serialization.
	out := render(root, unit{typ: uScalar, path: []int{99}}, markTheme())
	if strings.Contains(out, "«") {
		t.Errorf("no unit should be highlighted, got:\n%s", out)
	}
	if out != string(root.bytes()) {
		t.Errorf("unselected render should equal canonical bytes:\n got: %q\nwant: %q", out, string(root.bytes()))
	}
}

// TestRenderColorsEachTokenKind verifies the renderer dispatches each JSON
// token to the matching theme function — the basis for always-on syntax
// highlighting.
func TestRenderColorsEachTokenKind(t *testing.T) {
	th := renderTheme{
		key:   func(s string) string { return "K(" + s + ")" },
		str:   func(s string) string { return "S(" + s + ")" },
		num:   func(s string) string { return "N(" + s + ")" },
		lit:   func(s string) string { return "L(" + s + ")" },
		punct: func(s string) string { return "P(" + s + ")" },
		sel:   identity,
	}
	root, _ := parseTree([]byte(`{"name":"web01","port":8080,"on":true,"x":null}`))
	out := render(root, unit{typ: uScalar, path: []int{99}}, th) // nothing selected

	for _, want := range []string{
		`K("name")`, `S("web01")`, // key + string value
		`K("port")`, `N(8080)`, // key + number
		`L(true)`, `L(null)`, // bool + null
		`P({)`, `P(})`, `P(:)`, `P(,)`, // structural punctuation
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
}
