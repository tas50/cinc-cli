package jsoneditor

import (
	"strings"
	"testing"
)

// mark wraps highlighted text in guillemets so tests can assert on the
// selection without dealing with ANSI escape codes.
func mark(s string) string { return "«" + s + "»" }

func renderWith(t *testing.T, in string, u unit) string {
	t.Helper()
	root, err := parseTree([]byte(in))
	if err != nil {
		t.Fatalf("parseTree: %v", err)
	}
	return render(root, u, mark)
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
	// A unit that matches nothing leaves the document un-highlighted and
	// identical to the canonical serialization.
	out := render(root, unit{typ: uScalar, path: []int{99}}, mark)
	if strings.Contains(out, "«") {
		t.Errorf("no unit should be highlighted, got:\n%s", out)
	}
	if out != string(root.bytes()) {
		t.Errorf("unselected render should equal canonical bytes:\n got: %q\nwant: %q", out, string(root.bytes()))
	}
}
