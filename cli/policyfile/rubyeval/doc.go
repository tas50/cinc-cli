package rubyeval

// Canonical JSON schema
// =====================
//
// The shim (shim.rb) serializes a Policyfile's captured intent to this shape,
// which Evaluate unmarshals into EvaluatedPolicy. generate_goldens.rb produces
// the identical shape from chef-cli's own DSL so the two can be diffed:
//
//	{
//	  "name": "myapp" | null,
//	  "run_list": ["recipe[app::default]", "role[web]"],   // raw, as chef stores them
//	  "named_run_lists": {"update": ["recipe[app::update]"]},
//	  "default_source": [
//	    {"type": "community",   "uri": "https://supermarket.chef.io"},
//	    {"type": "chef_server", "uri": "https://chef.example.com/organizations/acme"},
//	    {"type": "chef_repo",   "path": "/cinc/cookbooks"}
//	  ],
//	  "cookbooks": {
//	    "app":    {"version_constraint": ">= 0.0.0", "source_options": {"path": "."}},
//	    "apache2":{"version_constraint": "~> 5.0",   "source_options": {}}
//	  },
//	  "included_policies": [{"name": "base", "source_options": {"path": "..."}}],
//	  "default_attributes":  {"app": {"port": 8080}},
//	  "override_attributes": {"app": {"env": "production"}},
//	  "errors": []
//	}
//
// chef semantics this mirrors (chef-cli 6.1.34, lib/chef-cli/policyfile/dsl.rb):
//
//   - run_list / named_run_list items are flattened and stored RAW (no
//     normalization to recipe[...]); the last non-empty run_list call wins.
//   - cookbook(name, *args): the trailing Hash is source_options; the FIRST
//     remaining argument is the version constraint (extra constraints are
//     dropped, exactly as chef does), defaulting to ">= 0.0.0". The constraint
//     is normalized by the (vendored, identical) Semverse gem: ">= 1.0" becomes
//     ">= 1.0.0" while "~> 2.0" is preserved.
//   - default_source :supermarket and :community both map to "community";
//     :chef_repo paths are expanded against a fixed policyfile-root sentinel so
//     results are portable.
//   - default[...] / override[...] assignments build a string-keyed,
//     autovivifying tree (chef's VividMash), so default['a']['b'] = c works.
//   - Validation/evaluation problems (empty run_list, invalid item names,
//     conflicting sources, a syntax error, or an exception the file raises) are
//     COLLECTED into "errors" rather than raised — matching chef's
//     eval_policyfile, which rescues and records.
//
// Evaluation vs resolution boundary
// =================================
//
// This package performs EVALUATION only: it runs the Policyfile.rb and records
// exactly what was declared. It deliberately does NOT perform dependency
// RESOLUTION — there is no version solving, no fetching from supermarket/git/
// chef_server, and no computation of chef-identical cookbook identifiers
// (content hashes) or cache keys. Those belong to a separate depsolver. The
// `cinc policy install` command builds on this evaluation and is explicit, in
// its output and help, about which parts of a Policyfile.lock.json it can and
// cannot fill in deterministically from evaluation alone.
