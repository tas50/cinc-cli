// Package nodeedit is the interactive editor behind `cinc node edit`. It
// presents the node's scalar fields (chef_environment, run_list,
// policy_name, policy_group) as a plain, non-JSON form edited in place, and
// surfaces the four Chef attribute precedence levels (normal, default,
// override, automatic) as rows that each open the shared, syntax-
// highlighted JSON editor (cli/jsoneditor) for one bag at a time.
//
// Splitting attributes out this way keeps the everyday edits — changing an
// environment or run-list — a single keystroke away while still giving the
// free-form attribute trees the full structural JSON editor.
package nodeedit

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	cinc "github.com/tas50/cinc-api"
)

// Run drives the node editor as a standalone full-screen program and
// returns the edited node. It is used by `cinc node edit`. An aborted edit
// returns an error, matching the JSON editor's contract.
func Run(in *cinc.Node) (*cinc.Node, error) {
	final, err := tea.NewProgram(runner{m: New(in)}, tea.WithAltScreen()).Run()
	if err != nil {
		return nil, fmt.Errorf("cinc: edit failed: %w", err)
	}
	r := final.(runner)
	if r.m.aborted {
		return nil, errors.New("cinc: edit aborted")
	}
	return r.m.Result(), nil
}

// runner adapts the embeddable Model to a standalone tea.Program, quitting
// once the user saves or aborts.
type runner struct{ m Model }

func (r runner) Init() tea.Cmd { return r.m.Init() }

func (r runner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	r.m, cmd = r.m.Update(msg)
	if r.m.finished {
		return r, tea.Quit
	}
	return r, cmd
}

func (r runner) View() string { return r.m.View() }
