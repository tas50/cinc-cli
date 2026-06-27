package policyfile

import (
	"regexp"

	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/policyfile/rubyeval"
)

// EvaluationLock builds the deterministic, evaluation-only portion of a
// Policyfile.lock.json from an evaluated Policyfile.
//
// It fills exactly what evaluating the Policyfile.rb determines on its own:
// the policy name, the run_list and named_run_lists (normalized to chef's
// recipe[...] lock form), the default/override attribute trees, and each
// cookbook's declared source options.
//
// It deliberately does NOT fill the fields that require dependency RESOLUTION —
// resolved cookbook versions, content identifiers, dotted-decimal identifiers,
// cache keys, solution dependencies, or included-policy locks. Those come from
// a depsolver (version solving + source fetching + chef-identical identifier
// computation), which is out of scope here. The returned lock is therefore an
// honest, partial artifact: see UnresolvedCookbooks for the cookbooks still
// awaiting resolution.
func EvaluationLock(eval *rubyeval.EvaluatedPolicy) *cinc.PolicyRevision {
	lock := &cinc.PolicyRevision{
		Name:               eval.Name,
		RunList:            normalizeRunList(eval.RunList),
		DefaultAttributes:  eval.DefaultAttributes,
		OverrideAttributes: eval.OverrideAttributes,
	}
	if len(eval.NamedRunLists) > 0 {
		lock.NamedRunLists = make(map[string][]string, len(eval.NamedRunLists))
		for name, items := range eval.NamedRunLists {
			lock.NamedRunLists[name] = normalizeRunList(items)
		}
	}
	if len(eval.Cookbooks) > 0 {
		lock.CookbookLocks = make(map[string]cinc.CookbookLock, len(eval.Cookbooks))
		for name, spec := range eval.Cookbooks {
			lock.CookbookLocks[name] = cinc.CookbookLock{
				SourceOptions: spec.SourceOptions,
			}
		}
	}
	return lock
}

// UnresolvedCookbooks returns the names of cookbooks in the evaluated policy
// whose version/identifier could not be filled from evaluation alone — which,
// since this package does no resolution, is all of them. Callers surface this
// so users understand the lock is not yet push-ready.
func UnresolvedCookbooks(eval *rubyeval.EvaluatedPolicy) []string {
	names := make([]string, 0, len(eval.Cookbooks))
	for name := range eval.Cookbooks {
		names = append(names, name)
	}
	return names
}

// runListItemRe matches a run_list item already wrapped in a recipe[...] or
// role[...] qualifier.
var runListItemRe = regexp.MustCompile(`^(recipe|role)\[.+\]$`)

// normalizeRunList rewrites bare run_list items to chef's lock form: a bare
// "app" or "app::svc" becomes "recipe[app]" / "recipe[app::svc]", while items
// already qualified as recipe[...] or role[...] are left untouched. This
// mirrors Chef::RunList::RunListItem#to_s, which is what chef writes into a
// Policyfile.lock.json.
func normalizeRunList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, len(items))
	for i, item := range items {
		if runListItemRe.MatchString(item) {
			out[i] = item
		} else {
			out[i] = "recipe[" + item + "]"
		}
	}
	return out
}
