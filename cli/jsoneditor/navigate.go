package jsoneditor

// unitType distinguishes the three things a structural cursor can land on.
type unitType uint8

const (
	uKey    unitType = iota // an object member's key
	uScalar                 // a scalar value (string/number/bool/null)
	uBlock                  // an object or array value, selected as a whole
)

// unit is one selectable target in the tree. path is the route from the
// root to the node, indexing object members or array elements at each
// level. For uKey, path points to the member whose key is selected; for
// uScalar/uBlock it points to the value node.
type unit struct {
	typ  unitType
	path []int
}

// at resolves a path to its node, or nil if the path does not exist.
func (root *node) at(path []int) *node {
	n := root
	for _, i := range path {
		switch n.kind {
		case kindObject:
			if i < 0 || i >= len(n.members) {
				return nil
			}
			n = n.members[i].val
		case kindArray:
			if i < 0 || i >= len(n.elems) {
				return nil
			}
			n = n.elems[i]
		default:
			return nil
		}
	}
	return n
}

// memberKeyAt returns the key of the object member that path points to.
func (root *node) memberKeyAt(path []int) (string, bool) {
	if len(path) == 0 {
		return "", false
	}
	parent := root.at(path[:len(path)-1])
	if parent == nil || parent.kind != kindObject {
		return "", false
	}
	i := path[len(path)-1]
	if i < 0 || i >= len(parent.members) {
		return "", false
	}
	return parent.members[i].key, true
}

// collectUnits walks the tree depth-first and returns every selectable
// unit in document order. A non-empty top-level block is not itself a
// unit — navigation starts on its first child — but an empty or scalar
// root yields a single unit so the cursor always has somewhere to sit.
func collectUnits(root *node) []unit {
	var us []unit

	var walkValue func(path []int, n *node)
	walkChildren := func(path []int, n *node) {
		switch n.kind {
		case kindObject:
			for i, m := range n.members {
				p := childPath(path, i)
				us = append(us, unit{typ: uKey, path: p})
				walkValue(p, m.val)
			}
		case kindArray:
			for i := range n.elems {
				p := childPath(path, i)
				walkValue(p, n.elems[i])
			}
		}
	}
	walkValue = func(path []int, n *node) {
		if n.kind == kindObject || n.kind == kindArray {
			us = append(us, unit{typ: uBlock, path: path})
			walkChildren(path, n)
		} else {
			us = append(us, unit{typ: uScalar, path: path})
		}
	}

	switch root.kind {
	case kindObject, kindArray:
		if childCount(root) == 0 {
			us = append(us, unit{typ: uBlock, path: nil})
		} else {
			walkChildren(nil, root)
		}
	default:
		us = append(us, unit{typ: uScalar, path: nil})
	}
	return us
}

// childPath returns a fresh path extending parent with index i. It copies
// so sibling paths never alias the same backing array.
func childPath(parent []int, i int) []int {
	p := make([]int, len(parent)+1)
	copy(p, parent)
	p[len(parent)] = i
	return p
}

func childCount(n *node) int {
	switch n.kind {
	case kindObject:
		return len(n.members)
	case kindArray:
		return len(n.elems)
	default:
		return 0
	}
}
