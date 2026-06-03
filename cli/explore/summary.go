package explore

import (
	"fmt"
	"strings"
	"time"

	cinc "github.com/tas50/cinc-api"
)

// summaryField is one label/value row in the right-hand summary panel.
type summaryField struct {
	Label string
	Value string
}

// summaryView is what the panel renders for the selected object: either a
// curated set of Fields, or — when a kind has no custom summary — the
// pretty-printed JSON in JSON.
type summaryView struct {
	Fields []summaryField
	JSON   string
}

// dash is the placeholder shown for an empty field value.
const dash = "—"

// orDash returns s, or the em-dash placeholder when s is empty.
func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return dash
	}
	return s
}

// relativeTime renders a Unix epoch (as a float, the shape Chef's
// ohai_time uses) as a short "Nh ago" string relative to now. A
// non-positive epoch means we never saw the node, and anything in the
// future collapses to "just now".
func relativeTime(epoch float64, now time.Time) string {
	if epoch <= 0 {
		return "never"
	}
	d := now.Sub(time.Unix(int64(epoch), 0))
	switch {
	case d < time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// nodeSummaryFields builds the curated facts panel for a node: the bits an
// operator scanning a fleet wants at a glance.
func nodeSummaryFields(n *cinc.Node, now time.Time) []summaryField {
	var ohai float64
	if v, ok := n.Automatic.Dig("ohai_time"); ok {
		if f, ok := v.(float64); ok {
			ohai = f
		}
	}
	return []summaryField{
		{"Environment", orDash(n.Environment)},
		{"Policy Group", orDash(n.PolicyGroup)},
		{"Client Version", orDash(n.Automatic.GetString("chef_packages", "chef", "version"))},
		{"Last Scan", relativeTime(ohai, now)},
		{"Run List", orDash(strings.Join(n.RunList, ", "))},
	}
}

// roleSummaryFields builds the curated facts panel for a role.
func roleSummaryFields(r *cinc.Role) []summaryField {
	return []summaryField{
		{"Description", orDash(r.Description)},
		{"Run List", orDash(strings.Join(r.RunList, ", "))},
		{"Env Run Lists", fmt.Sprintf("%d", len(r.EnvRunLists))},
	}
}

// environmentSummaryFields builds the curated facts panel for an
// environment.
func environmentSummaryFields(e *cinc.Environment) []summaryField {
	return []summaryField{
		{"Description", orDash(e.Description)},
		{"Cookbook Constraints", fmt.Sprintf("%d", len(e.CookbookVersions))},
	}
}
