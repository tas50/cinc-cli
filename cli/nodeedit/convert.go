// Package nodeedit is a bubbletea form for editing a Chef/Cinc node. It
// presents the node's name, environment, policy, and run list as plain
// human-readable fields and its attribute bags (normal, default, override,
// automatic) in the shared JSON editor — so the common parts of a node
// never look like raw JSON. The pure node<->form conversions live here;
// the interactive Model lives in model.go.
package nodeedit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	cinc "github.com/tas50/cinc-api"
)

// attrBags is the object presented in the attributes JSON editor: the four
// node attribute precedence levels, in Chef's conventional order.
type attrBags struct {
	Normal    cinc.Attributes `json:"normal"`
	Default   cinc.Attributes `json:"default"`
	Override  cinc.Attributes `json:"override"`
	Automatic cinc.Attributes `json:"automatic"`
}

// runListText renders a node's run list one entry per line for editing.
func runListText(n *cinc.Node) string { return strings.Join(n.RunList, "\n") }

// parseRunList turns edited run-list text back into entries, trimming each
// line and dropping blanks.
func parseRunList(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if e := strings.TrimSpace(line); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// attributesSeed is the JSON shown in the attributes editor: all four bags,
// in Chef's order, each defaulting to an empty object so every bag is
// visible. The JSON editor re-formats it, so the seed only needs valid,
// correctly-ordered JSON.
func attributesSeed(n *cinc.Node) ([]byte, error) {
	bags := []struct {
		key string
		val cinc.Attributes
	}{
		{"normal", n.Normal},
		{"default", n.Default},
		{"override", n.Override},
		{"automatic", n.Automatic},
	}
	var b bytes.Buffer
	b.WriteByte('{')
	for i, bag := range bags {
		if i > 0 {
			b.WriteByte(',')
		}
		v := map[string]any(bag.val)
		if v == nil {
			v = map[string]any{}
		}
		vb, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "%q:%s", bag.key, vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// buildNode assembles the edited form back into a node, carrying the name
// from the original (it is not editable). attrsJSON must contain exactly the
// four known bags; an unknown top-level key is rejected so a stray key is
// surfaced rather than silently dropped.
func buildNode(orig *cinc.Node, env, policyName, policyGroup string, runList []string, attrsJSON []byte) (*cinc.Node, error) {
	var bags attrBags
	dec := json.NewDecoder(bytes.NewReader(attrsJSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&bags); err != nil {
		return nil, err
	}
	return &cinc.Node{
		Name:        orig.Name,
		Environment: env,
		RunList:     runList,
		Normal:      bags.Normal,
		Default:     bags.Default,
		Override:    bags.Override,
		Automatic:   bags.Automatic,
		PolicyName:  policyName,
		PolicyGroup: policyGroup,
	}, nil
}

// nodeUnchanged reports whether two nodes are equivalent for edit purposes,
// treating nil and empty collections alike. The name is ignored since it is
// not editable.
func nodeUnchanged(a, b *cinc.Node) bool {
	return bytes.Equal(nodeSignature(a), nodeSignature(b))
}

func nodeSignature(n *cinc.Node) []byte {
	sig, _ := json.Marshal(map[string]any{
		"environment":  n.Environment,
		"policy_name":  n.PolicyName,
		"policy_group": n.PolicyGroup,
		"run_list":     orEmptySlice(n.RunList),
		"normal":       orEmptyMap(n.Normal),
		"default":      orEmptyMap(n.Default),
		"override":     orEmptyMap(n.Override),
		"automatic":    orEmptyMap(n.Automatic),
	})
	return sig
}

func orEmptyMap(a cinc.Attributes) map[string]any {
	if a == nil {
		return map[string]any{}
	}
	return a
}

func orEmptySlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
