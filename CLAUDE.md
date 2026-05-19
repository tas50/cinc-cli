# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project

`cinc` is a single, unified command-line tool for Cinc Infra (with full Chef
Infra compatibility) — one command with one consistent grammar. It is a Go
binary built with [Cobra](https://github.com/spf13/cobra).

## Project identity: cinc-first, chef-compatible

This tool is for **cinc**. Treat chef as a compatibility target, not the focus:

- **User-facing docs and examples** describe cinc flows first. Cinc-prefixed
  config keys, env vars, and file paths are the canonical form; the chef
  equivalents are mentioned as a compatibility note, not lead-with material.
- **Internal naming uses cinc.** Functions, types, packages, fixtures, and
  test names should say `cinc`, or be neutral, unless the symbol's job is
  specifically to handle chef-compat behavior — in which case the chef name
  is correct and clearer (e.g. `TestLoadAcceptsChefServerURLKey`).
- **Strive for backwards compatibility with existing Chef tools** (knife,
  chef workstation, chef-zero). Existing users should be able to point cinc
  at their existing `~/.chef/credentials`, env vars, and key files and have
  it work. **But every chef-prefixed config option or env var MUST also have
  a cinc-prefixed equivalent**, and when both are present the cinc form
  wins. The compatibility tables in README and tests are the source of
  truth for which knobs need a paired form.

Design documents live in `docs/`:

- `docs/*-cinc-cli-command-taxonomy.md` — the user-facing command structure
- `docs/*-cinc-cli-internal-architecture.md` — how the CLI is built

## Repository layout

- `apps/cinc/` — the binary.
  - `cinc.go` — thin `main()`; calls `cmd.Execute()`.
  - `cmd/` — the Cobra command tree. One file per noun group (`node.go`, …),
    plus `root.go` (root command, persistent flags) and `common.go`
    (flag-resolution helpers).
- `cli/` — reusable, independently-testable infrastructure:
  - `cli/config` — parses the TOML config file and resolves named profiles.
  - `cli/client` — builds a `cinc-api` client from a resolved profile.
  - `cli/printer` — renders command output as human text or JSON.
- `docs/` — design documents.

## Architecture rules

- **All server communication goes through `github.com/tas50/cinc-api`.** That
  library owns authentication (request signing), transport, and the API object
  model. The CLI never builds or signs an HTTP request. The single seam between
  CLI state and the API library is `cli/client`.
- Commands are **noun-verb**: `cinc <noun> <verb>` (e.g. `cinc node list`). The
  core verbs `list`/`show`/`create`/`edit`/`delete` mean the same thing on every
  noun.
- Keep the command layer thin — business logic belongs in the `cli/*` packages,
  which is where most tests live.

## Build & test

- `make build` — compile the binary; version metadata is injected via `-ldflags`.
- `make test` / `go test ./...` — run the test suite.
- `make vet`, `make fmt` — `go vet` and `gofmt`.
- `make help` — list all targets.

Acceptance tests (when present under `test/`) are gated behind the `acceptance`
build tag and run the real binary against a `chef-zero` server; they need Ruby
and the `chef-zero` gem:

```
go test -tags acceptance ./test/...
```

## Conventions

- **Test-driven development.** Write a failing test first, watch it fail for the
  expected reason, then write the minimal code to pass.
- Run `gofmt` and `go vet ./...` before committing; both must be clean.
- Configuration is a TOML file (`~/.cinc/credentials` by default) holding
  named profiles. Each top-level section is a profile carrying
  `cinc_server_url` (or, for chef compatibility, `chef_server_url`),
  `client_name`, `client_key`, and an optional `ssl_verify_mode`. The CLI
  splits the `/organizations/<org>` segment off the server URL internally.
  Profile selection: `--profile` flag → `$CINC_PROFILE` → `$CHEF_PROFILE` →
  `default`. The on-disk shape mirrors Chef's `~/.chef/credentials` so
  existing knife users can point cinc at their existing file unchanged.

## Adding a server command

1. Add (or extend) `apps/cinc/cmd/<noun>.go` with a `new<Noun>Cmd()` constructor.
2. Register it in `root.go` via `root.AddCommand(...)`.
3. Use `resolveClient(cmd)` and `resolveFormat(cmd)` from `common.go` to obtain a
   configured `cinc-api` client and the chosen output format.
4. Render results through `cli/printer` — never format output inline.
