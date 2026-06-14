# `cinc policy` lifecycle — thin verbs design

- **Date:** 2026-06-13
- **Status:** Approved
- **Scope:** The three server/local policy lifecycle verbs that need no
  Policyfile compiler or dependency solver: `create`, `diff`, `clean`.

## Background

The command taxonomy (`docs/dev/2026-05-19-cinc-cli-command-taxonomy.md`)
defines the full policy noun:

```
cinc policy  list  show  create  install  update  push  diff  export  delete  clean
```

`list`, `show`, and `delete` already exist. The remaining seven split into
two tiers:

- **Thin (this design):** `create` (scaffold a `Policyfile.rb` on disk),
  `diff` (compare revisions already on the server), `clean` (delete policy
  revisions no policy group references). Each is either pure local file I/O
  or thin `cinc-api` calls — the same shape as the existing CRUD verbs.
- **Heavy (deferred to a separate design):** `install`, `update`, `push`,
  `export`. All four hinge on compiling `Policyfile.rb` and solving cookbook
  dependencies into a `Policyfile.lock.json`. Design decision #6 bans a Ruby
  runtime, so these require a pure-Go Policyfile evaluator + depsolver — a
  major subsystem out of scope here.

## `cinc policy create <name>`

Scaffolds a Policyfile authoring source on disk. No server contact.

- Writes `<name>.rb` in the current directory (matches
  `chef generate policyfile NAME`).
- `--file <path>` overrides the output path.
- Refuses to overwrite an existing file; `--force` allows it.
- The `name '<name>'` directive inside matches the filename, so the on-server
  policy name and the future `<name>.lock.json` all key off the same `<name>`.

Template written:

```ruby
# <name>.rb - Describe how you want Cinc Infra Client to build your system.
#
# For more information on the Policyfile feature, see the Policyfile
# documentation: https://docs.chef.io/policyfile/   (Cinc is Policyfile-compatible)

# A name that describes what the system you're building with Cinc does.
name '<name>'

# Where to find external cookbooks:
default_source :supermarket

# run_list: Cinc Infra Client will run these recipes in the order specified.
run_list '<name>::default'

# Specify a custom source for a single cookbook:
# cookbook '<name>', path: '.'
```

Output: `Created Policyfile <name>.rb` (or the `--file` path).

## `cinc policy diff <name> ...`

Compares two policy revisions already on the server. Two forms:

1. **Across policy groups (default):** `cinc policy diff <name> <group1> <group2>`.
   Looks up which revision of `<name>` is active in each group
   (`PolicyGroups.GetPolicy`), fetches both revisions
   (`Policies.GetRevision`), and renders the delta.
2. **Explicit revisions:** `cinc policy diff <name> --revisions <revA> <revB>`.
   Fetches the two named revisions directly. The flag form keeps revision IDs
   from being mistaken for group names.

Exactly one form per invocation: with `--revisions`, no positional group args
are accepted; without it, exactly two group names are required.

The delta covers:

- **cookbook_locks** — added / removed / changed. A change reports the version
  when versions differ, otherwise the identifier.
- **run_list** — entries added / removed (order-insensitive set diff).
- **attributes** — `default_attributes` and `override_attributes` compared by
  flattened leaf path; reports added / removed / changed leaves.

Human output (text):

```
appserver: staging (1.1.0) -> prod (1.0.0)

  cookbook  web     1.3.0     -> 1.2.0
  cookbook  base    a1b2c3..  -> d4e5f6..
  run_list  + recipe[web::ssl]
  default   ['web']['port']   443 -> 8080
```

`--format json` emits the structured delta instead:

```json
{
  "policy": "appserver",
  "from": {"ref": "staging", "revision_id": "1.1.0"},
  "to":   {"ref": "prod",    "revision_id": "1.0.0"},
  "cookbooks": [{"name": "web", "from": "1.3.0", "to": "1.2.0"}],
  "run_list":  {"added": ["recipe[web::ssl]"], "removed": []},
  "attributes": [{"path": "default['web']['port']", "from": 443, "to": 8080}]
}
```

When the two revisions are identical the human form prints
`No differences.` and the JSON form reports empty deltas.

## `cinc policy clean [name]`

Deletes policy revisions that no policy group references (chef's
`clean-policy-revisions`).

- With no argument: sweeps every policy.
- With `<name>`: scopes the sweep to one policy.
- `--dry-run`: report what would be deleted without deleting.

Algorithm:

1. List policy groups (`PolicyGroups.List`); build the in-use set of
   `(policy, revision_id)` from each group's `policies` map.
2. List policies (`Policies.List`); for each revision not in the in-use set,
   `Policies.DeleteRevision` (unless `--dry-run`).

Output reports, per policy, which revisions were deleted (or would be) and how
many were kept because a group still uses them.

## Architecture

- New command file `apps/cinc/cmd/policylifecycle.go` holds the three
  constructors, registered in `newPolicyCmd()` in `policy.go`. (Keeping them
  out of `policy.go` keeps that file focused on the CRUD verbs.)
- `create` is pure `os` file I/O plus a template constant — no client.
- `diff` and `clean` go through `resolveClient`/`resolveFormat` and render via
  `cli/printer` (`diff` uses `Value` for the JSON delta and a small text
  renderer for the human form; `clean` uses `fmt.Fprintf` lines like the other
  mutating verbs).
- The diff computation (revision A, revision B → structured delta) is a pure
  function with no I/O, unit-tested directly.

## Testing

Per CLAUDE.md, every verb gets unit + acceptance coverage.

- **Unit (`apps/cinc/cmd/policylifecycle_test.go`):** drive each command
  through cobra against `httptest`. `create` writes to a `t.TempDir()` path via
  `--file` and asserts file contents + the overwrite/`--force` behavior. The
  pure diff function is unit-tested across added/removed/changed cookbooks, run
  lists, and attributes.
- **Acceptance (`test/acceptance/policylifecycle_test.go`):**
  - `create`: run the real binary with `--file <tmpdir>/x.rb`; assert the
    scaffold. No server needed (uses `buildCinc`, not `startAcceptance`).
  - `diff` / `clean`: the seed has a single in-use revision, so these tests
    create the extra state they need at runtime through a `cinc-api` client
    built from the acceptance profile (`Policies.CreateRevision`, and for the
    group form, `PolicyGroups.PutPolicy`) — no changes to the shared seed,
    which other tests depend on. `clean` asserts the orphaned revision is
    deleted and the in-use one is kept; `diff` asserts the rendered delta.
- `make docs` regenerates the per-command reference.
