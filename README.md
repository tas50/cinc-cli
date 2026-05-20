# cinc

`cinc` is a single, unified command-line tool for Cinc Infra (with full
Chef Infra compatibility) — one command with one consistent grammar. It
is a Go binary built with
[Cobra](https://github.com/spf13/cobra) on top of the
[`cinc-api`](https://github.com/tas50/cinc-api) client library, which owns
authentication and transport.

## Why Cinc CLI

The existing Chef toolchain works, but it carries a lot of historical
weight. `cinc` is a fresh take on the same job, with a few things we
weren't willing to compromise on:

- **Performance.** A native Go binary with no interpreter to spin up,
  so configuring a system or scripting against the server feels
  instant instead of sluggish.
- **Ease of installation.** A single static binary with no runtime
  dependencies. Nothing to break when the system Ruby is upgraded,
  no gem environment to babysit, and no multi-step bootstrap in your
  CI pipelines — drop the binary in and go.
- **Ease of setup.** `cinc config create` walks you through credentials,
  server URL, and SSL settings interactively, so a new user (or an
  existing Chef user pointing at a new server) is productive in
  minutes instead of losing an afternoon to troubleshooting.
- **Focused experience.** Scoped to day-to-day node and cookbook
  management against a Cinc/Chef server. No migration helpers, no
  one-off subcommands for legacy workflows — just the verbs you
  reach for every day, kept consistent across every noun.
- **Well documented.** Every command, subcommand, and flag has a
  generated Markdown reference under [`docs/commands/`](docs/commands/),
  rebuilt from the live cobra command tree on every change — so the
  docs can't drift from what the binary actually does.
- **Fully tested.** Every command ships with both unit tests against
  an in-process HTTP server and acceptance tests that run the real
  compiled binary against a live `chef-zero`, so behavior is verified
  end-to-end before it lands.

## Status

Early development. The command surface is being built out one noun
group at a time. See [`docs/commands/cinc.md`](docs/commands/cinc.md)
for the current command surface (auto-generated from the cobra command
tree). The taxonomy and internal architecture are described in
[`docs/dev/`](docs/dev/).

## Install

Requires Go 1.26 or newer.

```sh
# Build a local ./cinc binary
make build

# Or install into $GOBIN
make install
```

`make` injects version, commit, and build-date metadata into the binary
via `-ldflags`; `cinc version` will print them.

### Install in GitHub Actions

Tagged releases publish prebuilt Linux and macOS archives. A workflow can install
the Linux AMD64 binary without installing Go:

```yaml
- name: Install cinc
  env:
    CINC_VERSION: v0.1.0
  run: |
    archive="cinc_${CINC_VERSION}_linux_amd64.tar.gz"
    curl -fsSL -o "$archive" "https://github.com/tas50/cinc-cli/releases/download/${CINC_VERSION}/${archive}"
    tar -xzf "$archive"
    sudo install "cinc_${CINC_VERSION}_linux_amd64/cinc" /usr/local/bin/cinc
    cinc version
```

Release archives are built by `make dist` and include `SHA256SUMS`.

## Configure

`cinc` reads a TOML credentials file (default `~/.cinc/credentials`)
holding named profiles, in the same shape as Chef's `~/.chef/credentials`.
Each top-level section is a profile that points at one Cinc/Chef Server.

```toml
[default]
cinc_server_url = "https://cinc.example.com/organizations/acme"
client_name     = "tim"
client_key      = "/keys/tim.pem"

[staging]
cinc_server_url = "https://staging.example.com/organizations/acme-staging"
client_name     = "tim"
client_key      = "/keys/staging.pem"
ssl_verify_mode = ":verify_none"
```

Persistent flags on every command:

| Flag | Description |
| --- | --- |
| `--config` | Path to the credentials file (default `~/.cinc/credentials`) |
| `--profile` | Profile name to use (default: `$CINC_PROFILE`, then `$CHEF_PROFILE`, then `default`) |
| `--format` | Output format: `human` or `json` |

### `CINC_*` and `CHEF_*` are both accepted

Every chef-prefixed config key and environment variable has a
cinc-prefixed equivalent. Both are accepted; if both are set in the same
profile or environment the `cinc_`/`CINC_` form wins. This lets you run
`cinc` against an existing Chef setup unchanged, and override individual
settings without rewriting the whole file.

| Chef-prefixed | Cinc-prefixed |
| --- | --- |
| `chef_server_url` (TOML key) | `cinc_server_url` (TOML key) |
| `CHEF_PROFILE` (env var) | `CINC_PROFILE` (env var) |

## Usage

Commands are noun-verb. The core verbs (`list`, `show`, `create`,
`edit`, `delete`) mean the same thing on every noun. For a guided
walkthrough — profiles, output formats, common workflows — see
[`docs/README.md`](docs/README.md). For the exhaustive per-command
reference (every flag, every default), see
[`docs/commands/`](docs/commands/). The reference pages are
regenerated from the live cobra command tree by `make docs` and
refreshed automatically on every push to `main`.

## Develop

```sh
make test            # unit tests
make vet             # go vet
make fmt             # gofmt -w
make test-acceptance # acceptance tests against chef-zero (needs Ruby + chef-zero gem)
make help            # list all targets
```

Repository layout:

- `apps/cinc/` — the binary. `cinc.go` is a thin `main()`; `cmd/` holds
  the Cobra command tree (one file per noun group, plus `root.go` and
  flag-resolution helpers in `common.go`).
- `cli/config` — TOML config parsing and profile resolution.
- `cli/client` — builds a `cinc-api` client from a resolved profile. This
  is the single seam between CLI state and the API library; the CLI
  itself never builds or signs an HTTP request.
- `cli/printer` — renders command output as human text or JSON.
- `docs/` — auto-generated per-command reference under `commands/`
  (regenerated by `make docs`) and design docs under `dev/`.
- `tools/gendocs/` — small Go program that walks the cobra command
  tree and writes the Markdown reference. Invoked by `make docs`.
- `test/` — acceptance tests, gated behind the `acceptance` build tag.

See [`CLAUDE.md`](CLAUDE.md) for conventions followed when developing
with Claude Code.
