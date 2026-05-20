package cmd

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// editJSON renders a small bubbletea-based text editor on the
// terminal seeded with initial, and returns the buffer the user saved.
// It is a package variable so tests can override it without spinning
// up a TUI program. Ctrl-D saves; Esc / Ctrl-C aborts (returns an
// error so callers can distinguish "edited but unchanged" from
// "cancelled").
//
// cinc ships its own editor rather than shelling out to $VISUAL /
// $EDITOR so the experience is consistent across systems and works
// in minimal environments without an editor on PATH.
var editJSON = runBuiltinEditor

func runBuiltinEditor(initial []byte) ([]byte, error) {
	ta := textarea.New()
	ta.SetValue(string(initial))
	ta.SetWidth(120)
	ta.SetHeight(24)
	ta.ShowLineNumbers = true
	ta.Focus()
	// Default Ctrl-D in textarea deletes a character; we use it for
	// "done" so unbind it from the textarea's keymap.
	ta.KeyMap.DeleteCharacterForward.SetEnabled(false)

	m, err := tea.NewProgram(editorModel{textarea: ta}, tea.WithAltScreen()).Run()
	if err != nil {
		return nil, fmt.Errorf("cinc: built-in editor: %w", err)
	}
	mod := m.(editorModel)
	if mod.aborted {
		return nil, fmt.Errorf("cinc: edit aborted")
	}
	return []byte(mod.textarea.Value()), nil
}

// editorModel is the bubbletea program state for the built-in JSON
// editor.
type editorModel struct {
	textarea textarea.Model
	aborted  bool
}

func (m editorModel) Init() tea.Cmd { return textarea.Blink }

func (m editorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyCtrlD:
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.aborted = true
			return m, tea.Quit
		}
	}
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		// Leave a couple of rows for the header and footer.
		m.textarea.SetWidth(sz.Width)
		if sz.Height > 4 {
			m.textarea.SetHeight(sz.Height - 3)
		}
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m editorModel) View() string {
	return fmt.Sprintf(
		"cinc edit — Ctrl-D to save and submit, Esc/Ctrl-C to abort\n\n%s\n",
		m.textarea.View(),
	)
}
