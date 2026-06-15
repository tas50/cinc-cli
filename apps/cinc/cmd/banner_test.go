package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteWelcomeBanner(t *testing.T) {
	var buf bytes.Buffer
	writeWelcomeBanner(&buf)
	got := buf.String()

	if !strings.HasPrefix(got, "Welcome to\n") {
		t.Errorf("banner should lead with %q, got:\n%s", "Welcome to", got)
	}
	// A recognizable slice of the CINC wordmark appears below the lead-in.
	if !strings.Contains(got, "██████╗██║██║") {
		t.Errorf("banner missing the CINC wordmark, got:\n%s", got)
	}
}

func TestReadPromptLineDoesNotOverConsume(t *testing.T) {
	// Reading one line must leave the rest of the stream intact for the next
	// reader — the configure prompts read from the same stdin afterward.
	r := strings.NewReader("yes\nleftover\n")
	if got := readPromptLine(r); got != "yes" {
		t.Fatalf("first line = %q, want %q", got, "yes")
	}
	if got := readPromptLine(r); got != "leftover" {
		t.Errorf("second line = %q, want %q (input was over-consumed)", got, "leftover")
	}
}

func TestReadPromptLineTrimsAndHandlesEOFWithoutNewline(t *testing.T) {
	if got := readPromptLine(strings.NewReader("  n  ")); got != "n" {
		t.Errorf("readPromptLine = %q, want %q", got, "n")
	}
}
