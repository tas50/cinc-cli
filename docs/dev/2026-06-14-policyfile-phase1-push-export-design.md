# Policyfile support — Phase 1: `push` + `export` over an existing lock

- **Date:** 2026-06-14
- **Status:** Approved — ready for implementation planning
- **Scope:** Deploying an existing `Policyfile.lock.json` to a Cinc/Chef
  Server (`push`) and assembling a standalone bundle from it (`export`),
  including fetching+caching the cookbooks the lock names. **Out of scope:**
  the Policyfile compiler (`install`/`update` — DSL evaluation + dependency
  solving). That is a separate, later phase.

## Background

cinc today scaffolds a `Policyfile.rb` (`policy create`) and queries server
policy state (`policy list/show/diff/clean/delete`, `policy-group
list/show/delete`). It has no way to *deploy* a policy. The Chef ecosystem
verbs for that are `chef push` (upload cookbooks + associate a revision with a
policy group) and `chef export` (assemble a self-contained bundle for an
air-gapped `chef-client -z`).

A `Policyfile.lock.json` is self-describing: each `cookbook_locks` entry records
the cookbook's pinned `version`, content `identifier`, `dotted_decimal_identifier`,
`cache_key`, and `source_options` (where the cookbook came from). Because the
lock pins exact versions and sources, deploying it requires **no dependency
resolution** — only *fetching* the named cookbooks and uploading them. That is
what makes Phase 1 tractable without the solver.

## Key decisions

1. **cinc fetches and caches cookbooks itself**, from the sources the lock
   records — not from Chef Workstation's cache, and not requiring the user to
   pre-assemble cookbooks. Given only a lock, cinc retrieves everything. No Ruby
   / chef-cli dependency.
2. **Four source types** are supported: `artifactserver` (Supermarket, via
   cinc-supermarket), `path` (read in place), `git` (clone + checkout the locked
   revision), and `chef_server` (via cinc-api cookbook download). An unknown
   source produces a clear "unsupported source" error.
3. **Cache keyed by `cache_key`** under `~/.cinc/cache/cookbooks/<cache_key>/`,
   mirroring Chef's convention. `path:` cookbooks have no `cache_key` and are
   read in place (matching Chef). Cache hit = directory exists. This is an
   internal detail, not user-facing.
4. **Identifiers are trusted from the lock.** Each cookbook artifact is uploaded
   under the `identifier` the lock records, exactly as `chef push` does. No
   content-hashing in Phase 1 (that belongs to the compiler phase).
5. **`export` produces a Chef-compatible directory** with `--archive/-a` to also
   tar it.
6. **Architecture boundary:** the pure, reusable pieces go to **cinc-api**
   (which is dependency-free and must stay that way); the multi-source fetcher
   (which needs cinc-supermarket + git) lives in the **CLI** as the
   cross-library integration point.

## Architecture

### cinc-api (no new external dependencies)

- **Policyfile lock parse helpers** (`policyfile.go`): cinc-api's existing
  `PolicyRevision` + `CookbookLock` structs already model `Policyfile.lock.json`
  (Name, RevisionID, RunList, NamedRunLists, CookbookLocks, attributes,
  SolutionDependencies, IncludedPolicyLocks; and per-lock Version, Identifier,
  DottedDecimalIdentifier, CacheKey, Source, SourceOptions, SCMInfo). So no new
  model is needed — just `ParsePolicyfileLock([]byte) (*PolicyRevision, error)`
  and `LoadPolicyfileLock(path) (*PolicyRevision, error)` convenience helpers
  that unmarshal a lock file into a `PolicyRevision`.
- **`Policies.PushRevision(ctx, lock, group, cookbooks map[string]*LocalCookbook)`**:
  for each cookbook lock, `CookbookArtifacts.Upload(cookbooks[name],
  lock.CookbookLocks[name].Identifier)`; then `PolicyGroups.PutPolicy(group,
  lock.Name, lockDoc)`. Returns the created `*PolicyRevision`. The caller
  supplies already-located cookbooks, so cinc-api needs no knowledge of sources.

### cinc-cli — new `cli/policyfile` package

- **Fetcher/cache** (`fetch.go`, `cache.go`): `EnsureCookbook(ctx, lock
  CookbookLock) (dir string, err error)` — cache hit returns the cached dir;
  miss dispatches on `source_options` to one resolver each:
  - `resolveArtifactserver` — download+extract via cinc-supermarket.
  - `resolvePath` — return the path as-is (read in place; no cache copy).
  - `resolveGit` — clone the repo, checkout the locked revision, locate the
    cookbook subdirectory, copy into the cache.
  - `resolveChefServer` — download via cinc-api `Cookbooks.Download`.
  The git resolver shells out to the `git` binary (clone + checkout the
  pinned revision, copy the cookbook minus `.git`).
- **Export assembler** (`export.go`): `Export(lock, destDir, archive bool)` —
  writes `cookbooks/<name>-<dotted_decimal_identifier>/` (from the cache),
  `policies/<name>-<revision>.json`, the `Policyfile.lock.json`, and a generated
  client config suitable for `cinc-client -z`; tars the tree when `archive`.

### cinc-cli — commands (thin drivers)

- `cinc policy push <group> [lock]` (default `./Policyfile.lock.json`):
  parse lock → `EnsureCookbook` for each lock → build `map[name]*LocalCookbook`
  → `Policies.PushRevision`. Reports uploaded artifacts and the revision/group.
- `cinc policy export [lock] [dir] [--archive/-a]`: parse lock → ensure all
  cookbooks cached → assemble the tree.

## Data flow

```
push:   lock.json ──parse──▶ PolicyfileLock
                                   │ per cookbook_lock
                                   ▼
                        EnsureCookbook (cache hit? ──▶ dir)
                                   │ miss
                                   ▼  source_options discriminator
        artifactserver│path│git│chef_server ──▶ cached dir
                                   ▼
        PushRevision: Upload(cb, identifier)…  then PutPolicy(group, name, lock)
```

`export` shares the parse + `EnsureCookbook` steps, then assembles files on disk
instead of calling the server.

## Error handling

- Missing lock file → actionable message naming the expected path.
- Unknown / unsupported `source_options` → error naming the cookbook and the
  source type.
- Fetch failure (HTTP 404, git clone/checkout failure, network) → wrapped per
  cookbook so the user knows which one failed.
- `push` is **non-atomic but safe to retry**: artifact uploads are idempotent by
  identifier, so a failure during `PutPolicy` after some uploads can be re-run
  without harm. The command reports what was uploaded.
- An artifact identifier already present on the server is a no-op success (the
  upload protocol skips checksums the server already has).

## Testing

- **cinc-api:** table-driven lock-parse tests (including the seed lock and a
  cookbook-bearing lock); `PushRevision` against the `cinctest` harness,
  asserting the per-cookbook upload sequence and the final `PutPolicy`.
- **cinc-cli unit:** each resolver in isolation — `artifactserver` against an
  httptest Supermarket, `git` against a temporary local repo, `path` against a
  fixture cookbook dir, `chef_server` against an httptest server; the export
  assembler against a temp lock + cached cookbooks (assert tree + archive);
  `policy push`/`export` commands end-to-end against httptest.
- **cinc-cli acceptance (cinc-zero):** the seeded `appserver` policy has empty
  `cookbook_locks`, giving a trivial `push` (PutPolicy only) for free; a seeded
  small `path:`-sourced cookbook + lock exercises a real artifact-upload
  round-trip, verified with `policy show` / `policy-group show`. `export` is run
  and its on-disk tree asserted.

## Sequencing

1. **cinc-api PR:** lock model + `PushRevision` (+ tests). Tag a release.
2. **cinc-cli PR(s):** bump to that tag; add `cli/policyfile` (fetcher/cache +
   export) and the `push`/`export` commands, including all four source
   resolvers (path, artifactserver, chef_server, git).

Each `cinc <noun> <verb>` addition carries both unit and acceptance tests, per
the repository conventions, and `make docs` is regenerated.

## Out of scope (explicitly deferred)

- Policyfile compilation: `Policyfile.rb` DSL evaluation, dependency solving,
  lock generation (`install` / `update`).
- `undelete`, `clean-policy-cookbooks`, `show-policy` beyond what exists.
- Named-run-list *selection* logic beyond passing through what the lock records.
