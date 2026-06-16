package explore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The Users kind renders a curated facts panel in the summary pane, not
// raw JSON, so an operator scanning users sees the human details at a
// glance.
func TestUserKindSummaryShowsFields(t *testing.T) {
	mux := http.NewServeMux()
	jsonHandler(mux, "/users/alice", `{
		"username": "alice",
		"display_name": "Alice Liddell",
		"email": "alice@example.com",
		"first_name": "Alice",
		"last_name": "Liddell"
	}`)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	k, ok := newUserKind().(Summarizable)
	if !ok {
		t.Fatal("user kind is not Summarizable")
	}
	view, err := k.Summary(context.Background(), testClient(t, srv), "alice")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if view.JSON != "" {
		t.Errorf("user summary fell back to JSON: %q", view.JSON)
	}
	got := map[string]string{}
	for _, f := range view.Fields {
		got[f.Label] = f.Value
	}
	want := map[string]string{
		"Display Name": "Alice Liddell",
		"Email":        "alice@example.com",
		"First Name":   "Alice",
		"Last Name":    "Liddell",
	}
	for label, val := range want {
		if got[label] != val {
			t.Errorf("field %q = %q, want %q", label, got[label], val)
		}
	}
}

// userSummaryType drives the user kind's Summary against a server seeded
// with the given user and (optionally) an admins-group body, then returns
// the resolved "Type" field. An empty adminsBody leaves /groups/admins
// unhandled so the lookup fails.
func userSummaryType(t *testing.T, username, userBody, adminsBody string) string {
	t.Helper()
	mux := http.NewServeMux()
	jsonHandler(mux, "/users/"+username, userBody)
	if adminsBody != "" {
		jsonHandler(mux, "/organizations/acme/groups/admins", adminsBody)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	k := newUserKind().(Summarizable)
	view, err := k.Summary(context.Background(), testClient(t, srv), username)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	for _, f := range view.Fields {
		if f.Label == "Type" {
			return f.Value
		}
	}
	t.Fatalf("no Type field in summary: %+v", view.Fields)
	return ""
}

// A user who belongs to the org's admins group is labeled Administrator.
func TestUserKindSummaryAdministrator(t *testing.T) {
	got := userSummaryType(t,
		"alice", `{"username":"alice"}`,
		`{"groupname":"admins","users":["alice","bob"]}`)
	if got != "Administrator" {
		t.Errorf("Type = %q, want Administrator", got)
	}
}

// A user absent from the admins group is a plain User.
func TestUserKindSummaryPlainUser(t *testing.T) {
	got := userSummaryType(t,
		"carol", `{"username":"carol"}`,
		`{"groupname":"admins","users":["alice","bob"]}`)
	if got != "User" {
		t.Errorf("Type = %q, want User", got)
	}
}

// The pivotal bootstrap account is the server Superuser, decided by name
// without needing the admins-group lookup at all.
func TestUserKindSummaryPivotalSuperuser(t *testing.T) {
	got := userSummaryType(t, "pivotal", `{"username":"pivotal"}`, "")
	if got != "Superuser" {
		t.Errorf("Type = %q, want Superuser", got)
	}
}

// When the admins group can't be read (here it 404s because no handler is
// registered), admin status is reported as Unknown rather than guessed.
func TestUserKindSummaryUnknownWhenAdminsUnreadable(t *testing.T) {
	got := userSummaryType(t, "dave", `{"username":"dave"}`, "")
	if got != "Unknown" {
		t.Errorf("Type = %q, want Unknown", got)
	}
}
