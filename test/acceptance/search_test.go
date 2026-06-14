//go:build acceptance

package acceptance

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSearchNodeAgainstCincZero exercises `cinc search` against a real
// cinc-zero server's search index over the seeded nodes.
func TestSearchNodeAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	// id-only: the seeded node names.
	ids := runCinc(t, env.binary, "search", "node", "*:*", "-i", "--config", env.cfgPath)
	for _, want := range []string{"web01", "web02", "db01"} {
		if !strings.Contains(ids, want) {
			t.Errorf("search -i missing %q:\n%s", want, ids)
		}
	}

	// default table: header plus a matched node and the count footer.
	table := runCinc(t, env.binary, "search", "node", "name:web01", "--config", env.cfgPath)
	if !strings.Contains(table, "NAME") || !strings.Contains(table, "web01") {
		t.Errorf("search table missing header/row:\n%s", table)
	}
	if !strings.Contains(table, "matched") {
		t.Errorf("search table missing count footer:\n%s", table)
	}

	// json: structured result with a total.
	jsonOut := runCinc(t, env.binary, "search", "node", "name:web01", "--config", env.cfgPath, "--format", "json")
	var got struct {
		Total int               `json:"total"`
		Rows  []json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &got); err != nil {
		t.Fatalf("search (json) not valid JSON: %v\n%s", err, jsonOut)
	}
	if got.Total < 1 || len(got.Rows) < 1 {
		t.Errorf("search json = %+v, want at least one match", got)
	}
}

// TestSearchPartialAgainstCincZero exercises partial search (-a), where the
// requested attributes become the table columns.
func TestSearchPartialAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := runCinc(t, env.binary, "search", "node", "*:*", "-a", "name", "--config", env.cfgPath)
	if !strings.Contains(out, "NAME") {
		t.Errorf("partial search missing NAME column:\n%s", out)
	}
	for _, want := range []string{"web01", "web02", "db01"} {
		if !strings.Contains(out, want) {
			t.Errorf("partial search missing %q:\n%s", want, out)
		}
	}
}
