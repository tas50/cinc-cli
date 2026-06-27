package policyfile

import (
	"reflect"
	"sort"
	"testing"

	"github.com/tas50/cinc-cli/cli/policyfile/rubyeval"
)

func TestNormalizeRunList(t *testing.T) {
	got := normalizeRunList([]string{"app", "app::svc", "recipe[web::default]", "role[db]"})
	want := []string{"recipe[app]", "recipe[app::svc]", "recipe[web::default]", "role[db]"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalizeRunList = %v, want %v", got, want)
	}
	if normalizeRunList(nil) != nil {
		t.Error("empty run_list should normalize to nil")
	}
}

func TestEvaluationLockCapturesEvaluatedFields(t *testing.T) {
	eval := &rubyeval.EvaluatedPolicy{
		Name:          "shop",
		RunList:       []string{"shop", "role[web]"},
		NamedRunLists: map[string][]string{"update": {"shop::update"}},
		Cookbooks: map[string]rubyeval.CookbookSpec{
			"shop":  {VersionConstraint: ">= 0.0.0", SourceOptions: map[string]any{"path": "."}},
			"redis": {VersionConstraint: "~> 5.0", SourceOptions: map[string]any{}},
		},
		DefaultAttributes:  map[string]any{"shop": map[string]any{"port": 8080}},
		OverrideAttributes: map[string]any{"env": "prod"},
	}

	lock := EvaluationLock(eval)

	if lock.Name != "shop" {
		t.Errorf("name = %q", lock.Name)
	}
	if !reflect.DeepEqual(lock.RunList, []string{"recipe[shop]", "role[web]"}) {
		t.Errorf("run_list = %v (want normalized)", lock.RunList)
	}
	if !reflect.DeepEqual(lock.NamedRunLists["update"], []string{"recipe[shop::update]"}) {
		t.Errorf("named run list = %v", lock.NamedRunLists["update"])
	}
	if len(lock.CookbookLocks) != 2 {
		t.Fatalf("cookbook_locks = %v", lock.CookbookLocks)
	}
	if p, _ := lock.CookbookLocks["shop"].SourceOptions["path"].(string); p != "." {
		t.Errorf("shop source_options = %v", lock.CookbookLocks["shop"].SourceOptions)
	}
	// Evaluation does not resolve versions/identifiers; those stay empty.
	if lock.CookbookLocks["redis"].Version != "" || lock.CookbookLocks["redis"].Identifier != "" {
		t.Errorf("redis lock should carry no resolved version/identifier: %+v", lock.CookbookLocks["redis"])
	}
	if !reflect.DeepEqual(lock.DefaultAttributes, eval.DefaultAttributes) {
		t.Errorf("default attributes not carried through")
	}

	unresolved := UnresolvedCookbooks(eval)
	sort.Strings(unresolved)
	if !reflect.DeepEqual(unresolved, []string{"redis", "shop"}) {
		t.Errorf("unresolved cookbooks = %v, want all of them", unresolved)
	}
}
