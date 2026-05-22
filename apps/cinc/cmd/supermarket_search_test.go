package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestSupermarketSearchCommandRegistered(t *testing.T) {
	root := newRootCmd()
	sub, _, err := root.Find([]string{"supermarket", "search"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !strings.HasPrefix(sub.Use, "search") {
		t.Fatalf("Use = %q, want search", sub.Use)
	}
	for _, name := range []string{"supermarket-site", "limit", "verbose"} {
		if sub.Flags().Lookup(name) == nil {
			t.Fatalf("--%s flag missing", name)
		}
	}
}

func TestSupermarketSearchPrintsMatchingNames(t *testing.T) {
	srv := newSupermarketIndexServer(t, []indexCookbook{
		{Name: "nginx", Maintainer: "sous-chefs"},
		{Name: "nginx_simple", Maintainer: "someone"},
	}, nil)
	defer srv.Close()

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"supermarket", "search", "nginx", "--supermarket-site", srv.URL})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := buf.String(); got != "nginx\nnginx_simple\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestSupermarketSearchRequiresQuery(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"supermarket", "search"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when query missing")
	}
}
