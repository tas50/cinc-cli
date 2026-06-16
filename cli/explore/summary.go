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
// pretty-printed JSON in JSON. Title, when set, overrides the bare object
// name in the panel heading (a node adds its platform, for example).
type summaryView struct {
	Title  string
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

// nodeTitle is the heading for a node's summary panel: its name, plus the
// platform and version when ohai has reported them — e.g. "web1 - ubuntu
// 24.04". A node we've never scanned has no platform, so it shows the bare
// name.
func nodeTitle(n *cinc.Node) string {
	platform := n.Automatic.GetString("platform")
	if platform == "" {
		return n.Name
	}
	if version := n.Automatic.GetString("platform_version"); version != "" {
		return fmt.Sprintf("%s - %s %s", n.Name, platform, version)
	}
	return fmt.Sprintf("%s - %s", n.Name, platform)
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
		{"Run List", orDash(strings.Join(n.RunList, ", "))},
		{"Client Version", orDash(n.Automatic.GetString("chef_packages", "chef", "version"))},
		{"Last Scan", relativeTime(ohai, now)},
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

// userSummaryFields builds the curated facts panel for a user. The
// username is already the panel heading, so it leads with the user's
// Type — the access classification resolved by userType — followed by
// the human details an operator scans for.
func userSummaryFields(u *cinc.User, userType string) []summaryField {
	return []summaryField{
		{"Type", userType},
		{"Display Name", orDash(u.DisplayName)},
		{"Email", orDash(u.Email)},
		{"First Name", orDash(u.FirstName)},
		{"Last Name", orDash(u.LastName)},
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
