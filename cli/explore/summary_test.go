package explore

import (
	"testing"
	"time"

	cinc "github.com/tas50/cinc-api"
)

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	epoch := func(d time.Duration) float64 {
		return float64(now.Add(-d).Unix())
	}
	cases := []struct {
		name  string
		input float64
		want  string
	}{
		{"never zero", 0, "never"},
		{"never negative", -5, "never"},
		{"seconds", epoch(30 * time.Second), "30s ago"},
		{"minutes", epoch(5 * time.Minute), "5m ago"},
		{"hours", epoch(2 * time.Hour), "2h ago"},
		{"days", epoch(3 * 24 * time.Hour), "3d ago"},
		{"future", float64(now.Add(time.Hour).Unix()), "just now"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := relativeTime(tc.input, now); got != tc.want {
				t.Errorf("relativeTime(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNodeSummaryFields(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	n := &cinc.Node{
		Name:        "web01",
		Environment: "production",
		PolicyGroup: "web",
		RunList:     []string{"role[base]", "recipe[nginx]"},
		Automatic: cinc.Attributes{
			"ohai_time": float64(now.Add(-2 * time.Hour).Unix()),
			"chef_packages": map[string]any{
				"chef": map[string]any{"version": "18.4.2"},
			},
		},
	}
	fields := nodeSummaryFields(n, now)

	want := map[string]string{
		"Environment":    "production",
		"Policy Group":   "web",
		"Client Version": "18.4.2",
		"Last Scan":      "2h ago",
		"Run List":       "role[base], recipe[nginx]",
	}
	got := map[string]string{}
	for _, f := range fields {
		got[f.Label] = f.Value
	}
	for label, val := range want {
		if got[label] != val {
			t.Errorf("field %q = %q, want %q", label, got[label], val)
		}
	}
}

func TestNodeSummaryFieldsEmpty(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	n := &cinc.Node{Name: "bare"}
	got := map[string]string{}
	for _, f := range nodeSummaryFields(n, now) {
		got[f.Label] = f.Value
	}
	if got["Environment"] != "—" {
		t.Errorf("empty Environment = %q, want em dash", got["Environment"])
	}
	if got["Last Scan"] != "never" {
		t.Errorf("empty Last Scan = %q, want never", got["Last Scan"])
	}
	if got["Run List"] != "—" {
		t.Errorf("empty Run List = %q, want em dash", got["Run List"])
	}
}
