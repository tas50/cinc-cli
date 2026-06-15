package jsoneditor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// kind is the JSON type of a node.
type kind uint8

const (
	kindNull kind = iota
	kindBool
	kindNumber
	kindString
	kindArray
	kindObject
)

// node is one value in an ordered JSON tree. Unlike decoding into a
// map[string]any, object members keep their document order so the editor
// renders and re-serializes the document the way the user sees it.
//
// For scalars (null, bool, number, string) scalar holds the canonical
// JSON literal — e.g. `null`, `true`, `42`, `"web01"` — ready to write
// out verbatim. members is set for objects, elems for arrays.
type node struct {
	kind    kind
	scalar  string
	members []member
	elems   []*node
}

// member is a single ordered object entry. key is the raw (unquoted)
// key string; it is JSON-escaped when serialized.
type member struct {
	key string
	val *node
}

// parseTree decodes b into an ordered tree. It rejects malformed JSON and
// any trailing data after the top-level value.
func parseTree(b []byte) (*node, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	n, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("unexpected trailing data after JSON value")
	}
	return n, nil
}

// parseValue reads exactly one JSON value (and its children) from dec.
func parseValue(dec *json.Decoder) (*node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return parseFromToken(dec, tok)
}

func parseFromToken(dec *json.Decoder, tok json.Token) (*node, error) {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			n := &node{kind: kindObject}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyTok.(string)
				if !ok {
					return nil, fmt.Errorf("object key is not a string")
				}
				val, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				n.members = append(n.members, member{key: key, val: val})
			}
			if _, err := dec.Token(); err != nil { // closing '}'
				return nil, err
			}
			return n, nil
		case '[':
			n := &node{kind: kindArray}
			for dec.More() {
				el, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				n.elems = append(n.elems, el)
			}
			if _, err := dec.Token(); err != nil { // closing ']'
				return nil, err
			}
			return n, nil
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", t)
		}
	case string:
		return scalarNode(kindString, t)
	case json.Number:
		return &node{kind: kindNumber, scalar: t.String()}, nil
	case bool:
		return scalarNode(kindBool, t)
	case nil:
		return &node{kind: kindNull, scalar: "null"}, nil
	default:
		return nil, fmt.Errorf("unexpected token %T", tok)
	}
}

// scalarNode builds a scalar node whose literal is v marshaled to its
// canonical JSON form.
func scalarNode(k kind, v any) (*node, error) {
	lit, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &node{kind: k, scalar: string(lit)}, nil
}

// bytes serializes the tree to canonical pretty JSON (two-space indent),
// preserving object member order.
func (n *node) bytes() []byte {
	var b strings.Builder
	n.encode(&b, 0)
	return []byte(b.String())
}

const indentUnit = "  "

func (n *node) encode(b *strings.Builder, depth int) {
	switch n.kind {
	case kindObject:
		if len(n.members) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		inner := strings.Repeat(indentUnit, depth+1)
		for i, m := range n.members {
			b.WriteString(inner)
			b.Write(encodeKey(m.key))
			b.WriteString(": ")
			m.val.encode(b, depth+1)
			if i < len(n.members)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(indentUnit, depth))
		b.WriteByte('}')
	case kindArray:
		if len(n.elems) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		inner := strings.Repeat(indentUnit, depth+1)
		for i, el := range n.elems {
			b.WriteString(inner)
			el.encode(b, depth+1)
			if i < len(n.elems)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(indentUnit, depth))
		b.WriteByte(']')
	default:
		b.WriteString(n.scalar)
	}
}

// encodeKey returns the JSON-escaped, quoted form of an object key.
func encodeKey(key string) []byte {
	lit, err := json.Marshal(key)
	if err != nil { // keys are valid UTF-8 strings; marshal cannot fail
		return []byte(`""`)
	}
	return lit
}
