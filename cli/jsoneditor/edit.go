package jsoneditor

// This file holds the tree mutations behind the structural editor's four
// operations: edit a scalar value, edit (rename) a key, add a node, and
// delete a node. Editing a whole block reduces to replaceValueAt with a
// freshly parsed subtree. Every mutation validates its path and the kind
// of the parent it touches, returning false rather than panicking.

// replaceValueAt swaps the value node at path for n. An empty path
// replaces the root in place.
func (root *node) replaceValueAt(path []int, n *node) bool {
	if len(path) == 0 {
		*root = *n
		return true
	}
	parent, idx, ok := root.parentIndex(path)
	if !ok {
		return false
	}
	switch parent.kind {
	case kindObject:
		parent.members[idx].val = n
	case kindArray:
		parent.elems[idx] = n
	default:
		return false
	}
	return true
}

// setKeyAt renames the object member key at path.
func (root *node) setKeyAt(path []int, key string) bool {
	parent, idx, ok := root.parentIndex(path)
	if !ok || parent.kind != kindObject {
		return false
	}
	parent.members[idx].key = key
	return true
}

// deleteAt removes the member or element at path.
func (root *node) deleteAt(path []int) bool {
	parent, idx, ok := root.parentIndex(path)
	if !ok {
		return false
	}
	switch parent.kind {
	case kindObject:
		parent.members = append(parent.members[:idx], parent.members[idx+1:]...)
	case kindArray:
		parent.elems = append(parent.elems[:idx], parent.elems[idx+1:]...)
	default:
		return false
	}
	return true
}

// addMember appends key:val to the object at blockPath.
func (root *node) addMember(blockPath []int, key string, val *node) bool {
	block := root.at(blockPath)
	if block == nil || block.kind != kindObject {
		return false
	}
	block.members = append(block.members, member{key: key, val: val})
	return true
}

// addElem appends val to the array at blockPath.
func (root *node) addElem(blockPath []int, val *node) bool {
	block := root.at(blockPath)
	if block == nil || block.kind != kindArray {
		return false
	}
	block.elems = append(block.elems, val)
	return true
}

// insertSiblingAfter inserts a new member/element immediately after the
// node at path. key is used only when the parent is an object.
func (root *node) insertSiblingAfter(path []int, key string, val *node) bool {
	parent, idx, ok := root.parentIndex(path)
	if !ok {
		return false
	}
	switch parent.kind {
	case kindObject:
		parent.members = insertAt(parent.members, idx+1, member{key: key, val: val})
	case kindArray:
		parent.elems = insertAt(parent.elems, idx+1, val)
	default:
		return false
	}
	return true
}

// parentIndex resolves path to its parent node and the final index,
// verifying that the index is in range for that parent.
func (root *node) parentIndex(path []int) (*node, int, bool) {
	if len(path) == 0 {
		return nil, 0, false
	}
	parent := root.at(path[:len(path)-1])
	if parent == nil {
		return nil, 0, false
	}
	idx := path[len(path)-1]
	switch parent.kind {
	case kindObject:
		if idx < 0 || idx >= len(parent.members) {
			return nil, 0, false
		}
	case kindArray:
		if idx < 0 || idx >= len(parent.elems) {
			return nil, 0, false
		}
	default:
		return nil, 0, false
	}
	return parent, idx, true
}

// insertAt returns s with v spliced in at index i, without aliasing the
// original backing array's tail.
func insertAt[T any](s []T, i int, v T) []T {
	out := make([]T, 0, len(s)+1)
	out = append(out, s[:i]...)
	out = append(out, v)
	out = append(out, s[i:]...)
	return out
}
