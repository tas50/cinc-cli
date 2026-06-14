package components

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptPasswordNonTTYReadsLine(t *testing.T) {
	// A non-*os.File reader (pipe/redirect/test) takes the line-read fallback;
	// the secret is returned trimmed and is NOT echoed by us (the caller's
	// terminal would echo a TTY, which is why the TTY path uses ReadPassword).
	in := strings.NewReader("s3cr3t\n")
	var out bytes.Buffer
	got, err := PromptPassword(in, &out, "New password")
	if err != nil {
		t.Fatalf("PromptPassword: %v", err)
	}
	if got != "s3cr3t" {
		t.Errorf("PromptPassword = %q, want s3cr3t", got)
	}
	if !strings.Contains(out.String(), "New password") {
		t.Errorf("prompt label not written: %q", out.String())
	}
	// We must not echo the secret back ourselves.
	if strings.Contains(out.String(), "s3cr3t") {
		t.Errorf("PromptPassword echoed the secret: %q", out.String())
	}
}

func TestPromptPasswordEmptyInput(t *testing.T) {
	got, err := PromptPassword(strings.NewReader(""), &bytes.Buffer{}, "New password")
	if err != nil {
		t.Fatalf("PromptPassword: %v", err)
	}
	if got != "" {
		t.Errorf("PromptPassword on empty input = %q, want \"\"", got)
	}
}
