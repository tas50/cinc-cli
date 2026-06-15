package jsoneditor

import "strings"

// renderTheme styles each JSON token as it is written out. Every field
// wraps a token's text — production supplies lipgloss-colored styles for
// always-on syntax highlighting, while tests supply identity or marker
// functions. sel wraps the currently selected unit and takes precedence
// over the token's own color so the two never nest.
type renderTheme struct {
	key   func(string) string // object keys
	str   func(string) string // string values
	num   func(string) string // number values
	lit   func(string) string // bool / null values
	punct func(string) string // structural punctuation: {}[],:
	sel   func(string) string // the selected unit
}

// scalar styles a scalar literal by its JSON type.
func (th renderTheme) scalar(n *node) string {
	switch n.kind {
	case kindString:
		return th.str(n.scalar)
	case kindNumber:
		return th.num(n.scalar)
	default: // bool, null
		return th.lit(n.scalar)
	}
}

// render serializes the tree to pretty JSON, coloring each token via th
// and wrapping the selected unit with th.sel. A uKey wraps just the key
// token, a uScalar wraps the value literal, and a uBlock wraps the entire
// object/array from its opening bracket through its closing bracket.
func render(root *node, sel unit, th renderTheme) string {
	var b strings.Builder
	renderValue(&b, root, nil, 0, sel, th)
	return b.String()
}

func renderValue(b *strings.Builder, n *node, path []int, depth int, sel unit, th renderTheme) {
	// A selected block or scalar value is wrapped as a whole.
	if sel.typ != uKey && pathEq(sel.path, path) {
		if n.kind == kindObject || n.kind == kindArray {
			var tmp strings.Builder
			n.encode(&tmp, depth)
			b.WriteString(th.sel(tmp.String()))
			return
		}
		b.WriteString(th.sel(n.scalar))
		return
	}

	switch n.kind {
	case kindObject:
		renderObject(b, n, path, depth, sel, th)
	case kindArray:
		renderArray(b, n, path, depth, sel, th)
	default:
		b.WriteString(th.scalar(n))
	}
}

func renderObject(b *strings.Builder, n *node, path []int, depth int, sel unit, th renderTheme) {
	if len(n.members) == 0 {
		b.WriteString(th.punct("{}"))
		return
	}
	b.WriteString(th.punct("{"))
	b.WriteByte('\n')
	inner := strings.Repeat(indentUnit, depth+1)
	for i, m := range n.members {
		p := childPath(path, i)
		b.WriteString(inner)
		key := string(encodeKey(m.key))
		if sel.typ == uKey && pathEq(sel.path, p) {
			b.WriteString(th.sel(key))
		} else {
			b.WriteString(th.key(key))
		}
		b.WriteString(th.punct(":"))
		b.WriteByte(' ')
		renderValue(b, m.val, p, depth+1, sel, th)
		if i < len(n.members)-1 {
			b.WriteString(th.punct(","))
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(indentUnit, depth))
	b.WriteString(th.punct("}"))
}

func renderArray(b *strings.Builder, n *node, path []int, depth int, sel unit, th renderTheme) {
	if len(n.elems) == 0 {
		b.WriteString(th.punct("[]"))
		return
	}
	b.WriteString(th.punct("["))
	b.WriteByte('\n')
	inner := strings.Repeat(indentUnit, depth+1)
	for i, el := range n.elems {
		p := childPath(path, i)
		b.WriteString(inner)
		renderValue(b, el, p, depth+1, sel, th)
		if i < len(n.elems)-1 {
			b.WriteString(th.punct(","))
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(indentUnit, depth))
	b.WriteString(th.punct("]"))
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
