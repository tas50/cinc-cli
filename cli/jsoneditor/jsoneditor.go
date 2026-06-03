// Package jsoneditor is a small bubbletea component that edits a JSON
// document in a textarea, validates it on save, and shows a
// syntax-highlighted preview before committing. It is shared by the
// `cinc <noun> edit` commands (via Run, which drives it as a standalone
// program) and by the `cinc explore` TUI (which embeds the Model as a
// screen).
//
// cinc ships its own editor rather than shelling out to $VISUAL /
// $EDITOR so the experience is consistent across systems and works in
// minimal environments without an editor on PATH.
package jsoneditor

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Model is the editor state. It is an embeddable bubbletea component:
// call New, then forward Update/View to it, and check Finished after
// each Update to learn when the user has committed or aborted. It does
// not quit the surrounding program — embedding code decides what to do
// when Finished reports true.
type Model struct {
	textarea  textarea.Model
	validate  func([]byte) error
	errMsg    string
	pending   []byte // canonical pretty JSON, set when preview is shown
	preview   string // highlighted preview, non-empty in preview mode
	committed []byte // set when the user confirms the preview
	aborted   bool
	finished  bool // committed or aborted — embedding code polls this
}

// New returns an editor seeded with initial. validate is invoked on
// each save attempt; returning a non-nil error keeps the user editing
// with the message shown inline (no work lost).
func New(initial []byte, validate func([]byte) error) Model {
	ta := textarea.New()
	ta.SetValue(string(initial))
	ta.SetWidth(120)
	ta.SetHeight(20)
	ta.ShowLineNumbers = true
	// Default Ctrl-D in textarea deletes a character; we bind it to
	// "save" so disable the textarea's binding.
	ta.KeyMap.DeleteCharacterForward.SetEnabled(false)
	ta.Focus()
	return Model{textarea: ta, validate: validate}
}

func (m Model) Init() tea.Cmd { return textarea.Blink }

// SetSize resizes the textarea to fit the given terminal dimensions,
// reserving a few lines for the header.
func (m *Model) SetSize(width, height int) {
	m.textarea.SetWidth(width)
	if height > 6 {
		m.textarea.SetHeight(height - 5)
	}
}

// Update advances the editor. It returns the concrete Model (not
// tea.Model) so embedding code can call Finished/Committed without a
// type assertion.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.SetSize(sz.Width, sz.Height)
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		if m.preview != "" {
			switch k.Type {
			case tea.KeyEnter, tea.KeyCtrlD:
				m.committed = m.pending
				m.finished = true
				return m, nil
			case tea.KeyEsc, tea.KeyCtrlC:
				// Drop preview, return to editing.
				m.preview = ""
				m.pending = nil
				return m, nil
			}
			// Ignore other keys in preview mode.
			return m, nil
		}
		switch k.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.aborted = true
			m.finished = true
			return m, nil
		case tea.KeyCtrlD:
			content := []byte(m.textarea.Value())
			if err := m.validate(content); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
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
			m.pending = pretty
			m.preview = highlightJSON(pretty)
			return m, nil
		default:
			// Any other key means the user is typing — clear the stale
			// error message so it doesn't linger after a fix.
			m.errMsg = ""
		}
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.preview != "" {
		return "Preview — Enter or Ctrl-D to confirm, Esc to keep editing\n\n" + m.preview
	}
	header := "cinc edit — Ctrl-D to validate & preview, Esc/Ctrl-C abort\n"
	if m.errMsg != "" {
		header += "Error: " + m.errMsg + "\n"
	}
	header += "\n"
	return header + m.textarea.View()
}

// Finished reports whether the user has committed or aborted.
func (m Model) Finished() bool { return m.finished }

// Aborted reports whether the user abandoned the edit.
func (m Model) Aborted() bool { return m.aborted }

// Committed returns the canonical JSON the user confirmed, or nil if
// the edit was aborted or is still in progress.
func (m Model) Committed() []byte { return m.committed }

// Run drives the editor as a standalone full-screen program and returns
// the committed JSON. It is used by the `cinc <noun> edit` commands.
func Run(initial []byte, validate func([]byte) error) ([]byte, error) {
	final, err := tea.NewProgram(runner{m: New(initial, validate)}, tea.WithAltScreen()).Run()
	if err != nil {
		return nil, fmt.Errorf("cinc: edit failed: %w", err)
	}
	r := final.(runner)
	if r.m.aborted {
		return nil, fmt.Errorf("cinc: edit aborted")
	}
	return r.m.committed, nil
}

// runner adapts the embeddable Model to a standalone tea.Program,
// quitting once the user finishes.
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

// highlightJSON wraps b in chroma's terminal256 JSON highlighting. If
// any step fails the raw bytes are returned so the preview is still
// readable.
func highlightJSON(b []byte) string {
	lexer := lexers.Get("json")
	if lexer == nil {
		return string(b)
	}
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		return string(b)
	}
	iterator, err := lexer.Tokenise(nil, string(b))
	if err != nil {
		return string(b)
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return string(b)
	}
	return buf.String()
}
