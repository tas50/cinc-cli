package rubyeval

// EvaluatedPolicy is the fully-evaluated intent of a Policyfile.rb, captured by
// running the file through the embedded Ruby DSL shim. It mirrors the canonical
// JSON the shim emits (see shim.rb / doc.go) and the state chef-cli's
// ChefCLI::Policyfile::DSL records for the same file.
//
// This is the *evaluation* result — the declared intent — not a resolved lock.
// Version solving, source fetching, and identifier computation are a separate
// concern (see the policy install command and the package docs for the exact
// evaluation-vs-resolution boundary).
type EvaluatedPolicy struct {
	// Name is the policy name (the `name` declaration). Empty if unset.
	Name string `json:"name"`

	// RunList is the primary run_list, stored exactly as chef stores it: the
	// raw item strings as written (e.g. "recipe[app::default]", "role[web]"),
	// not normalized to recipe[...] form.
	RunList []string `json:"run_list"`

	// NamedRunLists holds each named_run_list keyed by its name (symbols are
	// stringified, matching chef).
	NamedRunLists map[string][]string `json:"named_run_lists"`

	// DefaultSources lists default_source declarations in order. Each carries a
	// "type" (community/chef_server/chef_repo/artifactory/delivery_supermarket)
	// plus the located option ("uri" or, for chef_repo, an expanded "path").
	DefaultSources []DefaultSource `json:"default_source"`

	// Cookbooks maps cookbook name to its declared version constraint and
	// source options. The constraint is normalized exactly as chef's Semverse
	// does (e.g. ">= 1.0" -> ">= 1.0.0", "~> 2.0" stays "~> 2.0"). Note chef
	// keeps only the FIRST constraint when several are passed.
	Cookbooks map[string]CookbookSpec `json:"cookbooks"`

	// IncludedPolicies lists include_policy declarations.
	IncludedPolicies []IncludedPolicy `json:"included_policies"`

	// DefaultAttributes is the deep-merged tree built from default[...] = ...
	// assignments (string-keyed, autovivified), matching chef's VividMash.
	DefaultAttributes map[string]any `json:"default_attributes"`

	// OverrideAttributes is the same for override[...] = ... assignments.
	OverrideAttributes map[string]any `json:"override_attributes"`

	// Errors holds the validation/evaluation problems chef would record for
	// this file (empty run_list, invalid item names, conflicting sources, a
	// syntax error, or an exception raised by the file). chef does not raise on
	// these — it collects them — and neither does the shim.
	Errors []string `json:"errors"`
}

// DefaultSource is one default_source declaration.
type DefaultSource struct {
	// Type is the semantic source type: "community", "chef_server",
	// "chef_repo", "artifactory", or "delivery_supermarket". ":supermarket" and
	// ":community" both map to "community" (chef treats them identically).
	Type string `json:"type"`
	// URI is the source URI for community/chef_server/artifactory/
	// delivery_supermarket sources.
	URI string `json:"uri,omitempty"`
	// Path is the expanded chef-repo path for chef_repo sources.
	Path string `json:"path,omitempty"`
}

// CookbookSpec is one cookbook's declared constraint and source options.
type CookbookSpec struct {
	// VersionConstraint is the Semverse-normalized constraint string.
	VersionConstraint string `json:"version_constraint"`
	// SourceOptions holds per-cookbook source options (path:, git:, ref:,
	// branch:, tag:, …) with symbol keys stringified.
	SourceOptions map[string]any `json:"source_options"`
}

// IncludedPolicy is one include_policy declaration.
type IncludedPolicy struct {
	Name          string         `json:"name"`
	SourceOptions map[string]any `json:"source_options"`
}
