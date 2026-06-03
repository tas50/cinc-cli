package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupermarketDownloadCommandRegistered(t *testing.T) {
	root := newRootCmd()
	sub, _, err := root.Find([]string{"supermarket", "download"})
	if err != nil {
		t.Fatalf("Find supermarket download: %v", err)
	}
	if sub.Use == "" || !strings.HasPrefix(sub.Use, "download") {
		t.Fatalf("Use = %q, want download", sub.Use)
	}
	for _, name := range []string{"file", "force", "supermarket-site"} {
		if sub.Flags().Lookup(name) == nil {
			t.Fatalf("--%s flag missing", name)
		}
	}
}

func TestSupermarketDownloadWritesTarballAndReportsToHuman(t *testing.T) {
	srv := newCommandDownloadServer(t, "nginx", "1.2.0", []byte("tar-body"))
	defer srv.Close()

	dir := t.TempDir()
	out := filepath.Join(dir, "nginx.tgz")

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"supermarket", "download", "nginx", "1.2.0",
		"--file", out,
		"--supermarket-site", srv.URL,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("supermarket download: %v", err)
	}
	got := buf.String()
	want := fmt.Sprintf("Downloaded nginx 1.2.0 to %s\n", out)
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(body) != "tar-body" {
		t.Fatalf("tarball = %q", body)
	}
}

func TestSupermarketDownloadDefaultsFilenameToCookbookDashVersion(t *testing.T) {
	srv := newCommandDownloadServer(t, "nginx", "1.2.0", []byte("tar-body"))
	defer srv.Close()

	dir := t.TempDir()
	t.Chdir(dir)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"supermarket", "download", "nginx", "1.2.0",
		"--supermarket-site", srv.URL,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("supermarket download: %v", err)
	}
	want := filepath.Join(dir, "nginx-1.2.0.tar.gz")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected default filename %s: %v", want, err)
	}
}

func TestSupermarketDownloadJSONOutput(t *testing.T) {
	srv := newCommandDownloadServer(t, "nginx", "1.2.0", []byte("tar-body"))
	defer srv.Close()

	dir := t.TempDir()
	out := filepath.Join(dir, "nginx.tgz")

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"supermarket", "download", "nginx", "1.2.0",
		"--file", out,
		"--supermarket-site", srv.URL,
		"--format", "json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("supermarket download --format json: %v", err)
	}
	var result struct {
		Cookbook string `json:"cookbook"`
		Version  string `json:"version"`
		File     string `json:"file"`
		Bytes    int64  `json:"bytes"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, buf.String())
	}
	if result.Cookbook != "nginx" || result.Version != "1.2.0" || result.File != out || result.Bytes != int64(len("tar-body")) {
		t.Fatalf("result = %+v", result)
	}
}

func TestSupermarketDownloadVersionDefaultsToLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/cookbooks/nginx/versions/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"version":"4.5.6"}`)
		case strings.HasSuffix(r.URL.Path, "/download"):
			_, _ = w.Write([]byte("tar-body"))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"supermarket", "download", "nginx",
		"--file", dir,
		"--supermarket-site", srv.URL,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("supermarket download: %v", err)
	}
	want := filepath.Join(dir, "nginx-4.5.6.tar.gz")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected file %s: %v", want, err)
	}
	if !strings.Contains(buf.String(), "Downloaded nginx 4.5.6 to "+want) {
		t.Fatalf("output = %q, want resolved version", buf.String())
	}
}

func TestSupermarketDownloadForceOverwrites(t *testing.T) {
	srv := newCommandDownloadServer(t, "nginx", "1.2.0", []byte("fresh"))
	defer srv.Close()

	dir := t.TempDir()
	out := filepath.Join(dir, "nginx.tgz")
	if err := os.WriteFile(out, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Without --force we should refuse.
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"supermarket", "download", "nginx", "1.2.0",
		"--file", out, "--supermarket-site", srv.URL,
	})
	if err := root.Execute(); err == nil {
		t.Fatal("expected refusal without --force")
	}
	got, _ := os.ReadFile(out)
	if string(got) != "stale" {
		t.Fatalf("file modified without --force: %q", got)
	}

	// With --force we should overwrite.
	root = newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{
		"supermarket", "download", "nginx", "1.2.0",
		"--file", out, "--force", "--supermarket-site", srv.URL,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("supermarket download --force: %v", err)
	}
	got, _ = os.ReadFile(out)
	if string(got) != "fresh" {
		t.Fatalf("file contents = %q after --force, want fresh", got)
	}
}

func newCommandDownloadServer(t *testing.T, cookbook, version string, body []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "/api/v1/cookbooks/" + cookbook + "/versions/" + strings.ReplaceAll(version, ".", "_") + "/download"
		if r.URL.Path != want {
			t.Errorf("unexpected request %s %s, want %s", r.Method, r.URL.Path, want)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(body)
	}))
}
