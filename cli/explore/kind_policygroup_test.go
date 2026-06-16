package explore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A policy group shows how many policies it pins on hover.
func TestPolicyGroupKindSummary(t *testing.T) {
	mux := http.NewServeMux()
	jsonHandler(mux, "/organizations/acme/policy_groups/prod",
		`{"policies":{"base":{"revision_id":"abc"},"web":{"revision_id":"def"}}}`)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	view, err := policyGroupKind{}.Summary(context.Background(), testClient(t, srv), "prod")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	got := map[string]string{}
	for _, f := range view.Fields {
		got[f.Label] = f.Value
	}
	if got["Policies"] != "2" {
		t.Errorf("Policies = %q, want 2", got["Policies"])
	}
}

// A policy-group binding shows the pinned revision and what it runs.
func TestPGPoliciesKindSummary(t *testing.T) {
	mux := http.NewServeMux()
	jsonHandler(mux, "/organizations/acme/policy_groups/prod/policies/base",
		`{"revision_id":"abc123","run_list":["recipe[nginx]"]}`)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	k := pgPoliciesKind{group: "prod"}
	view, err := k.Summary(context.Background(), testClient(t, srv), "base")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	got := map[string]string{}
	for _, f := range view.Fields {
		got[f.Label] = f.Value
	}
	if got["Revision"] != "abc123" {
		t.Errorf("Revision = %q, want abc123", got["Revision"])
	}
	if got["Run List"] != "recipe[nginx]" {
		t.Errorf("Run List = %q, want recipe[nginx]", got["Run List"])
	}
}
