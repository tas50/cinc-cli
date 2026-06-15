package jsoneditor

import "strings"

// render serializes the tree to pretty JSON and wraps the text of the
// selected unit with hl. A uKey wraps just the key token, a uScalar wraps
// the value literal, and a uBlock wraps the entire object/array from its
// opening bracket through its closing bracket. If the unit matches no node
// the output equals the canonical serialization.
func render(root *node, sel unit, hl func(string) string) string {
	var b strings.Builder
	renderValue(&b, root, nil, 0, sel, hl)
	return b.String()
}

// renderValue writes n at the given path/depth, applying hl where the
// selection lands.
func renderValue(b *strings.Builder, n *node, path []int, depth int, sel unit, hl func(string) string) {
	// A selected block or scalar value is wrapped as a whole.
	if sel.typ != uKey && pathEq(sel.path, path) {
		if n.kind == kindObject || n.kind == kindArray {
			var tmp strings.Builder
			n.encode(&tmp, depth)
			b.WriteString(hl(tmp.String()))
			return
		}
		b.WriteString(hl(n.scalar))
		return
	}

	switch n.kind {
	case kindObject:
		renderObject(b, n, path, depth, sel, hl)
	case kindArray:
		renderArray(b, n, path, depth, sel, hl)
	default:
		b.WriteString(n.scalar)
	}
}

func renderObject(b *strings.Builder, n *node, path []int, depth int, sel unit, hl func(string) string) {
	if len(n.members) == 0 {
		b.WriteString("{}")
		return
	}
	b.WriteString("{\n")
	inner := strings.Repeat(indentUnit, depth+1)
	for i, m := range n.members {
		p := childPath(path, i)
		b.WriteString(inner)
		key := string(encodeKey(m.key))
		if sel.typ == uKey && pathEq(sel.path, p) {
			key = hl(key)
		}
		b.WriteString(key)
		b.WriteString(": ")
		renderValue(b, m.val, p, depth+1, sel, hl)
		if i < len(n.members)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(indentUnit, depth))
	b.WriteByte('}')
}

func renderArray(b *strings.Builder, n *node, path []int, depth int, sel unit, hl func(string) string) {
	if len(n.elems) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteString("[\n")
	inner := strings.Repeat(indentUnit, depth+1)
	for i, el := range n.elems {
		p := childPath(path, i)
		b.WriteString(inner)
		renderValue(b, el, p, depth+1, sel, hl)
		if i < len(n.elems)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(indentUnit, depth))
	b.WriteByte(']')
}

func pathEq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
