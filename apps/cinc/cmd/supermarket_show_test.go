package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupermarketShowCommandRegistered(t *testing.T) {
	root := newRootCmd()
	sub, _, err := root.Find([]string{"supermarket", "show"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !strings.HasPrefix(sub.Use, "show") {
		t.Fatalf("Use = %q, want show", sub.Use)
	}
}

func TestSupermarketShowCookbookKeyValueOutput(t *testing.T) {
	srv := newSupermarketShowServer(t, map[string]string{
		"/api/v1/cookbooks/nginx": `{
			"name": "nginx",
			"maintainer": "sous-chefs",
			"description": "Installs and configures NGINX",
			"category": "Web Servers",
			"latest_version": "https://example.test/api/v1/cookbooks/nginx/versions/12_0_4",
			"external_url": "https://github.com/sous-chefs/nginx",
			"versions": [
				"https://example.test/api/v1/cookbooks/nginx/versions/12_0_4",
				"https://example.test/api/v1/cookbooks/nginx/versions/12_0_3"
			],
			"metrics": {"downloads": {"total": 89341221}, "followers": 0, "collaborators": 0}
		}`,
	})
	defer srv.Close()

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"supermarket", "show", "nginx", "--supermarket-site", srv.URL})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := buf.String()
	wants := []string{
		"Name:",
		"nginx",
		"Maintainer:",
		"sous-chefs",
		"Latest version:",
		"12.0.4",
		"Downloads:",
		"89,341,221",
		"Source URL:",
		"https://github.com/sous-chefs/nginx",
		"Versions:",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Fatalf("output missing %q\noutput:\n%s", w, got)
		}
	}
}

func TestSupermarketShowVersionKeyValueOutput(t *testing.T) {
	srv := newSupermarketShowServer(t, map[string]string{
		"/api/v1/cookbooks/nginx/versions/12_0_4": `{
			"version": "12.0.4",
			"license": "Apache-2.0",
			"tarball_file_size": 145842,
			"dependencies": {"ohai": ">= 5.2.0"},
			"supports": {"ubuntu": ">= 0.0.0"}
		}`,
	})
	defer srv.Close()

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"supermarket", "show", "nginx", "12.0.4", "--supermarket-site", srv.URL})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"Version:", "12.0.4", "License:", "Apache-2.0", "Dependencies:", "ohai", ">= 5.2.0", "Platforms:", "ubuntu"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q\noutput:\n%s", want, got)
		}
	}
}

func TestSupermarketShowJSONEmitsShowResult(t *testing.T) {
	srv := newSupermarketShowServer(t, map[string]string{
		"/api/v1/cookbooks/nginx": `{"name": "nginx", "maintainer": "sous-chefs"}`,
	})
	defer srv.Close()

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"supermarket", "show", "nginx", "--format", "json", "--supermarket-site", srv.URL})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var decoded struct {
		Cookbook *struct {
			Name string `json:"name"`
		} `json:"cookbook"`
		Version *struct {
			Version string `json:"version"`
		} `json:"version"`
	}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("json: %v\noutput: %s", err, buf.String())
	}
	if decoded.Cookbook == nil || decoded.Cookbook.Name != "nginx" {
		t.Fatalf("Cookbook = %+v", decoded.Cookbook)
	}
	if decoded.Version != nil {
		t.Fatalf("Version should be nil for cookbook-level show")
	}
}

func newSupermarketShowServer(t *testing.T, responses map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := responses[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
}
