package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
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

// editDataBagItem opens a data bag item in the JSON editor.
// Validation rejects malformed JSON and items missing an "id" key.
// It is a package variable so tests can stub it.
var editDataBagItem = openDataBagItemJSONEditor

func openDataBagItemJSONEditor(in cinc.DataBagItem) (cinc.DataBagItem, error) {
	initial, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, err
	}
	edited, err := runJSONEditor(initial, validateDataBagItem)
	if err != nil {
		return nil, err
	}
	var out cinc.DataBagItem
	if err := json.Unmarshal(edited, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// runJSONEditor opens a TUI textarea seeded with initial. On Ctrl-D
// the buffer is parsed, validated, and reformatted. If validation
// fails the error is shown inline and the user keeps editing — no
// work is lost. On success the editor switches into a preview mode
// that shows the canonical JSON with syntax highlighting; Enter (or
// another Ctrl-D) confirms and exits, Esc returns to editing.
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
// returns nil the buffer is auto-formatted and the model switches
// into preview mode (pending non-empty), otherwise errMsg is set and
// the user keeps editing.
type jsonEditorModel struct {
	textarea  textarea.Model
	validate  func([]byte) error
	errMsg    string
	pending   []byte // canonical pretty JSON, set when preview is shown
	preview   string // highlighted preview, non-empty in preview mode
	committed []byte // set when the user confirms the preview
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
		if m.preview != "" {
			switch k.Type {
			case tea.KeyEnter, tea.KeyCtrlD:
				m.committed = m.pending
				return m, tea.Quit
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
			return m, tea.Quit
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
