package jsoneditor

import "testing"

func mustParse(t *testing.T, s string) *node {
	t.Helper()
	n, err := parseTree([]byte(s))
	if err != nil {
		t.Fatalf("parseTree(%q): %v", s, err)
	}
	return n
}

func TestReplaceValueAtScalar(t *testing.T) {
	root := mustParse(t, `{"a":1}`)
	if !root.replaceValueAt([]int{0}, mustParse(t, `2`)) {
		t.Fatal("replaceValueAt returned false")
	}
	if got := string(root.bytes()); got != "{\n  \"a\": 2\n}" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceValueAtBlock(t *testing.T) {
	root := mustParse(t, `{"l":["a"]}`)
	if !root.replaceValueAt([]int{0}, mustParse(t, `["x","y"]`)) {
		t.Fatal("replaceValueAt returned false")
	}
	want := "{\n  \"l\": [\n    \"x\",\n    \"y\"\n  ]\n}"
	if got := string(root.bytes()); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestReplaceValueAtRoot(t *testing.T) {
	root := mustParse(t, `{"a":1}`)
	if !root.replaceValueAt(nil, mustParse(t, `"scalar"`)) {
		t.Fatal("root replace returned false")
	}
	if got := string(root.bytes()); got != `"scalar"` {
		t.Errorf("got %q", got)
	}
}

func TestSetKeyAt(t *testing.T) {
	root := mustParse(t, `{"a":1}`)
	if !root.setKeyAt([]int{0}, "b") {
		t.Fatal("setKeyAt returned false")
	}
	if got := string(root.bytes()); got != "{\n  \"b\": 1\n}" {
		t.Errorf("got %q", got)
	}
}

func TestDeleteAtObject(t *testing.T) {
	root := mustParse(t, `{"a":1,"b":2}`)
	if !root.deleteAt([]int{0}) {
		t.Fatal("deleteAt returned false")
	}
	if got := string(root.bytes()); got != "{\n  \"b\": 2\n}" {
		t.Errorf("got %q", got)
	}
}

func TestDeleteAtArray(t *testing.T) {
	root := mustParse(t, `[1,2,3]`)
	if !root.deleteAt([]int{1}) {
		t.Fatal("deleteAt returned false")
	}
	if got := string(root.bytes()); got != "[\n  1,\n  3\n]" {
		t.Errorf("got %q", got)
	}
}

func TestAddMember(t *testing.T) {
	root := mustParse(t, `{}`)
	if !root.addMember(nil, "x", mustParse(t, `null`)) {
		t.Fatal("addMember returned false")
	}
	if got := string(root.bytes()); got != "{\n  \"x\": null\n}" {
		t.Errorf("got %q", got)
	}
}

func TestAddElem(t *testing.T) {
	root := mustParse(t, `[]`)
	if !root.addElem(nil, mustParse(t, `true`)) {
		t.Fatal("addElem returned false")
	}
	if got := string(root.bytes()); got != "[\n  true\n]" {
		t.Errorf("got %q", got)
	}
}

func TestInsertSiblingAfterKeepsOrder(t *testing.T) {
	root := mustParse(t, `{"a":1,"c":3}`)
	if !root.insertSiblingAfter([]int{0}, "b", mustParse(t, `2`)) {
		t.Fatal("insertSiblingAfter returned false")
	}
	want := "{\n  \"a\": 1,\n  \"b\": 2,\n  \"c\": 3\n}"
	if got := string(root.bytes()); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestInsertSiblingAfterArray(t *testing.T) {
	root := mustParse(t, `[1,3]`)
	if !root.insertSiblingAfter([]int{0}, "", mustParse(t, `2`)) {
		t.Fatal("insertSiblingAfter returned false")
	}
	if got := string(root.bytes()); got != "[\n  1,\n  2,\n  3\n]" {
		t.Errorf("got %q", got)
	}
}

func TestMutationsRejectBadPaths(t *testing.T) {
	root := mustParse(t, `{"a":1}`)
	if root.deleteAt([]int{5}) {
		t.Error("deleteAt out-of-range should fail")
	}
	if root.setKeyAt([]int{0, 0}, "x") {
		t.Error("setKeyAt into a scalar should fail")
	}
	if root.addElem(nil, mustParse(t, `1`)) {
		t.Error("addElem on an object should fail")
	}
}
