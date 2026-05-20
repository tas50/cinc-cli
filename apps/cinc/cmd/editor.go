package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	cinc "github.com/tas50/cinc-api"
)

// editClient presents the supplied client as pretty-printed JSON in
// a small textarea editor and returns the parsed result. It is a
// package variable so tests can override it without spawning a TUI.
//
// cinc ships its own editor rather than shelling out to $VISUAL /
// $EDITOR so the experience is consistent across systems and works
// in minimal environments without an editor on PATH.
var editClient = openClientJSONEditor

func openClientJSONEditor(in *cinc.APIClient) (*cinc.APIClient, error) {
	initial, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, err
	}
	edited, err := runJSONEditor(initial, func(b []byte) error {
		var c cinc.APIClient
		return json.Unmarshal(b, &c)
	})
	if err != nil {
		return nil, err
	}
	var out cinc.APIClient
	if err := json.Unmarshal(edited, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// runJSONEditor opens a TUI textarea seeded with initial. On Ctrl-D
// it parses, validates, and reformats the buffer. If parsing or the
// caller-supplied validate fails, the error is shown inline and the
// user stays in the editor so nothing they typed is lost. On success
// the editor returns the canonical pretty-printed form of the value.
// Esc / Ctrl-C aborts with an error.
func runJSONEditor(initial []byte, validate func([]byte) error) ([]byte, error) {
	ta := textarea.New()
	ta.SetValue(string(initial))
	ta.SetWidth(120)
	ta.SetHeight(20)
	ta.ShowLineNumbers = true
	// Default Ctrl-D in textarea deletes a character; we bind it to
	// "save" so disable the textarea's binding.
	ta.KeyMap.DeleteCharacterForward.SetEnabled(false)
	ta.Focus()

	final, err := tea.NewProgram(
		jsonEditorModel{textarea: ta, validate: validate},
		tea.WithAltScreen(),
	).Run()
	if err != nil {
		return nil, fmt.Errorf("cinc: edit failed: %w", err)
	}
	m := final.(jsonEditorModel)
	if m.aborted {
		return nil, fmt.Errorf("cinc: edit aborted")
	}
	return m.committed, nil
}

// jsonEditorModel is the bubbletea program state for the JSON
// textarea editor. validate is invoked on each save attempt; when it
// returns nil the buffer is auto-formatted and committed, otherwise
// errMsg is set and the user keeps editing.
type jsonEditorModel struct {
	textarea  textarea.Model
	validate  func([]byte) error
	errMsg    string
	committed []byte
	aborted   bool
}

func (m jsonEditorModel) Init() tea.Cmd { return textarea.Blink }

func (m jsonEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.textarea.SetWidth(sz.Width)
		if sz.Height > 6 {
			m.textarea.SetHeight(sz.Height - 5)
		}
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.aborted = true
			return m, tea.Quit
		case tea.KeyCtrlD:
			content := []byte(m.textarea.Value())
			if err := m.validate(content); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			// Validation passed — reformat into canonical pretty-
			// printed JSON so the caller gets a stable shape.
			var generic any
			if err := json.Unmarshal(content, &generic); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			pretty, err := json.MarshalIndent(generic, "", "  ")
			if err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.committed = pretty
			return m, tea.Quit
		default:
			// Any other key means the user is typing — clear the
			// stale error message so it doesn't linger after a fix.
			m.errMsg = ""
		}
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m jsonEditorModel) View() string {
	header := "cinc edit — Ctrl-D save & submit, Esc/Ctrl-C abort\n"
	if m.errMsg != "" {
		header += "Error: " + m.errMsg + "\n"
	}
	header += "\n"
	return header + m.textarea.View()
}
