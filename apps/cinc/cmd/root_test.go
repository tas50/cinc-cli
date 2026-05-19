package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandRunsVersionSubcommand(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc version failed: %v", err)
	}

	if !strings.HasPrefix(buf.String(), "cinc ") {
		t.Errorf("expected version output from `cinc version`, got:\n%s", buf.String())
	}
}
