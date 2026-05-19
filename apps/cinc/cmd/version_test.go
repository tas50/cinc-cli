package cmd

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestVersionInfoStringIncludesAllFields(t *testing.T) {
	info := versionInfo{
		Version:   "1.2.3",
		Commit:    "abc1234",
		BuildDate: "2026-05-19",
		GoVersion: "go1.26.3",
		Platform:  "darwin/arm64",
	}

	out := info.String()

	for _, want := range []string{"1.2.3", "abc1234", "2026-05-19", "go1.26.3", "darwin/arm64"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestNewVersionInfoPopulatesRuntimeFields(t *testing.T) {
	info := newVersionInfo()

	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", info.GoVersion, runtime.Version())
	}
	wantPlatform := runtime.GOOS + "/" + runtime.GOARCH
	if info.Platform != wantPlatform {
		t.Errorf("Platform = %q, want %q", info.Platform, wantPlatform)
	}
}

func TestVersionCommandWritesVersionOutput(t *testing.T) {
	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, runtime.Version()) {
		t.Errorf("version command output missing Go version %q\ngot:\n%s", runtime.Version(), out)
	}
}
