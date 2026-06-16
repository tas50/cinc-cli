package explore

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Editing a node in the explorer opens the human-field form (the same one
// `cinc node edit` uses), not the raw JSON editor.
func TestEditNodeOpensForm(t *testing.T) {
	mux := http.NewServeMux()
	jsonHandler(mux, "/organizations/acme/nodes", `{"web01":"u"}`)
	jsonHandler(mux, "/organizations/acme/nodes/web01", `{
		"name": "web01",
		"chef_environment": "production",
		"run_list": ["role[base]"]
	}`)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	m := modelOnKinds(t, testClient(t, srv))
	m, cmd := pressKey(t, m, tea.KeyEnter) // open Nodes
	m, _ = step(t, m, drain(cmd))
	m, cmd = pressRune(t, m, 'e') // start edit
	m, _ = step(t, m, drain(cmd)) // editSeedMsg → editor opens
	if _, ok := m.editor.(*nodeFormAdapter); !ok {
		t.Fatalf("editing a node should open the node form, got %T", m.editor)
	}
	out := m.View()
	for _, want := range []string{"Node: web01", "Environment", "Run list", "Attributes"} {
		if !strings.Contains(out, want) {
			t.Errorf("form view missing %q\n%s", want, out)
		}
	}
}

// Creating a node opens the same form with an editable name, and committing
// it POSTs the node the user filled in.
func TestCreateNodeUsesForm(t *testing.T) {
	var posted string
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/nodes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			posted = string(b)
			_, _ = w.Write([]byte(`{"uri":"https://example.test/nodes/web02"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`)) // list + reload
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	m := modelOnKinds(t, testClient(t, srv))
	m, cmd := pressKey(t, m, tea.KeyEnter) // open Nodes
	m, _ = step(t, m, drain(cmd))

	m, _ = m2(m.startCreate()) // node form, focused on the editable name
	if _, ok := m.editor.(*nodeFormAdapter); !ok {
		t.Fatalf("creating a node should open the node form, got %T", m.editor)
	}
	if !strings.Contains(m.View(), "New node") {
		t.Errorf("create form should head with %q\n%s", "New node", m.View())
	}
	for _, r := range "web02" {
		m, _ = pressRune(t, m, r)
	}
	m, cmd = pressKey(t, m, tea.KeyCtrlD) // commit
	m, _ = step(t, m, drain(cmd))         // mutationDoneMsg
	if !strings.Contains(posted, `"name":"web02"`) {
		t.Errorf("POST body = %q, want it to carry name web02", posted)
	}
	if !strings.Contains(m.status, "Created web02") {
		t.Errorf("status = %q, want Created web02", m.status)
	}
}

// An empty name keeps the create form open with an error instead of POSTing.
func TestCreateNodeRequiresName(t *testing.T) {
	var posted bool
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/nodes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	m := modelOnKinds(t, testClient(t, srv))
	m, cmd := pressKey(t, m, tea.KeyEnter)
	m, _ = step(t, m, drain(cmd))

	m, _ = m2(m.startCreate())
	m, _ = pressKey(t, m, tea.KeyCtrlD) // commit with no name typed
	if posted {
		t.Error("an empty name must not POST a node")
	}
	if m.screen != screenEditor {
		t.Errorf("screen = %v, want the form to stay open", m.screen)
	}
}
