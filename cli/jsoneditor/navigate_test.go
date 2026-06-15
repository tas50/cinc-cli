package jsoneditor

import (
	"fmt"
	"testing"
)

// fmtUnit renders a unit compactly so the expected sequence reads clearly.
func fmtUnit(u unit) string {
	name := map[unitType]string{uKey: "key", uScalar: "scalar", uBlock: "block"}[u.typ]
	return fmt.Sprintf("%s%v", name, u.path)
}

func TestCollectUnitsOrderAndPaths(t *testing.T) {
	root, err := parseTree([]byte(`{"name":"web01","run_list":["a","b"],"tags":{}}`))
	if err != nil {
		t.Fatalf("parseTree: %v", err)
	}
	got := []string{}
	for _, u := range collectUnits(root) {
		got = append(got, fmtUnit(u))
	}
	want := []string{
		"key[0]",      // "name"
		"scalar[0]",   // "web01"
		"key[1]",      // "run_list"
		"block[1]",    // the [ ... ] array, whole
		"scalar[1 0]", // "a"
		"scalar[1 1]", // "b"
		"key[2]",      // "tags"
		"block[2]",    // empty object, whole
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("unit sequence mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func TestCollectUnitsScalarRoot(t *testing.T) {
	root, _ := parseTree([]byte(`"hello"`))
	us := collectUnits(root)
	if len(us) != 1 || us[0].typ != uScalar || len(us[0].path) != 0 {
		t.Errorf("scalar root should yield one scalar unit at the empty path, got %v", us)
	}
}

func TestCollectUnitsEmptyArrayRoot(t *testing.T) {
	root, _ := parseTree([]byte(`[]`))
	us := collectUnits(root)
	if len(us) != 1 || us[0].typ != uBlock {
		t.Errorf("empty array root should yield one block unit, got %v", us)
	}
}

func TestNodeAtResolvesPath(t *testing.T) {
	root, _ := parseTree([]byte(`{"a":{"b":["x","y"]}}`))
	n := root.at([]int{0, 0, 1}) // a -> b -> elem[1]
	if n == nil || n.kind != kindString || n.scalar != `"y"` {
		t.Errorf("at([0 0 1]) = %+v, want string \"y\"", n)
	}
}

func TestMemberKeyAtResolvesKey(t *testing.T) {
	root, _ := parseTree([]byte(`{"name":"web01","run_list":[]}`))
	key, ok := root.memberKeyAt([]int{1})
	if !ok || key != "run_list" {
		t.Errorf("memberKeyAt([1]) = %q,%v want run_list,true", key, ok)
	}
}
