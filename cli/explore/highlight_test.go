package explore

import (
	"regexp"
	"strings"
	"testing"
)

// sgr matches the SGR color escape sequences chroma's terminal formatter
// emits, so a test can strip them back to plain text.
var sgr = regexp.MustCompile("\x1b\\[[0-9;]*m")

const sampleJSON = "{\n  \"name\": \"web01\",\n  \"run_list\": [\n    \"recipe[nginx]\"\n  ]\n}"

// colorizeJSON must wrap the JSON in terminal color codes without altering a
// single character of the underlying text — stripping the codes returns the
// exact input, so the detail pane stays faithful and copy-pasteable.
func TestColorizeJSONHighlightsButPreservesText(t *testing.T) {
	out := colorizeJSON(sampleJSON)
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("colorizeJSON added no color codes: %q", out)
	}
	if got := sgr.ReplaceAllString(out, ""); got != sampleJSON {
		t.Errorf("stripped output != input\n got: %q\nwant: %q", got, sampleJSON)
	}
}

// highlightJSON is the gated entry point: when color is off (pipes, tests,
// NO_COLOR) it must return the text untouched so plain output stays clean.
func TestHighlightJSONPlainWhenDisabled(t *testing.T) {
	if got := highlightJSON(false, sampleJSON); got != sampleJSON {
		t.Errorf("highlightJSON(false) = %q; want unchanged input", got)
	}
}

// When color is on, highlightJSON colorizes.
func TestHighlightJSONColorsWhenEnabled(t *testing.T) {
	out := highlightJSON(true, sampleJSON)
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("highlightJSON(true) added no color codes: %q", out)
	}
}

// In a non-interactive environment (the test process is not a TTY) the color
// probe must report false, so explore never injects escape codes into piped
// or captured output.
func TestColorizeDisabledWithoutTTY(t *testing.T) {
	if colorize(newStyles()) {
		t.Error("colorize reported true in a non-TTY test environment")
	}
}

// With color enabled, opening a node's detail must render the JSON with
// syntax-highlight escape codes in the visible viewport — the highlighting
// has to reach the screen, not just the helper.
func TestDetailViewHighlightsWhenColorEnabled(t *testing.T) {
	srv, _ := countingNodeServer(t)
	m, _ := openNodes(t, srv)
	m.colorJSON = true

	next, cmd := m.openSelected()
	m = next.(model)
	m, _ = step(t, m, drain(cmd)) // describeCmd → detailLoadedMsg → content set

	view := m.detail.View()
	if !strings.Contains(view, "\x1b[") {
		t.Errorf("detail view is not syntax-highlighted:\n%q", view)
	}
	if !strings.Contains(view, "web01") {
		t.Errorf("highlighting dropped the node content:\n%s", view)
	}
}

// With color disabled the detail view must stay plain — no escape codes — so
// piped or non-TTY sessions show raw JSON.
func TestDetailViewPlainWhenColorDisabled(t *testing.T) {
	srv, _ := countingNodeServer(t)
	m, _ := openNodes(t, srv)
	m.colorJSON = false

	next, cmd := m.openSelected()
	m = next.(model)
	m, _ = step(t, m, drain(cmd))

	if view := m.detail.View(); strings.Contains(view, "\x1b[") {
		t.Errorf("detail view should be plain when color is off:\n%q", view)
	}
}
