# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project

`cinc` is a single, unified command-line tool for Chef/Cinc Infra — one command
with one consistent grammar. It is a Go binary built with [Cobra](https://github.com/spf13/cobra).

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
- Configuration is a TOML file (`~/.cinc/config.toml` by default) holding named
  profiles. Each profile carries `server_url`, `org`, `client_name`, `key_path`.

## Adding a server command

1. Add (or extend) `apps/cinc/cmd/<noun>.go` with a `new<Noun>Cmd()` constructor.
2. Register it in `root.go` via `root.AddCommand(...)`.
3. Use `resolveClient(cmd)` and `resolveFormat(cmd)` from `common.go` to obtain a
   configured `cinc-api` client and the chosen output format.
4. Render results through `cli/printer` — never format output inline.
