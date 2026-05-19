# cinc

`cinc` is a single, unified command-line tool for Chef/Cinc Infra — one
command with one consistent grammar. It is a Go binary built with
[Cobra](https://github.com/spf13/cobra) on top of the
[`cinc-api`](https://github.com/tas50/cinc-api) client library, which owns
authentication and transport.

## Status

Early development. The command surface is being built out one noun group
at a time. Today: `cinc version`, `cinc node list`. The taxonomy and
internal architecture are described in [`docs/`](docs/).

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

## Configure

`cinc` reads a TOML config file (default `~/.cinc/config.toml`) holding
named profiles. Each profile points at one Chef/Cinc Server.

```toml
default_profile = "prod"

[profiles.prod]
server_url  = "https://chef.example.com"
org         = "acme"
client_name = "tim"
key_path    = "/keys/tim.pem"

[profiles.staging]
server_url  = "https://staging.example.com"
org         = "acme-staging"
client_name = "tim"
key_path    = "/keys/staging.pem"
```

Persistent flags on every command:

| Flag | Description |
| --- | --- |
| `--config` | Path to the config file (default `~/.cinc/config.toml`) |
| `--profile` | Profile name to use (default: the config's `default_profile`) |
| `--format` | Output format: `human` or `json` |

## Usage

Commands are noun-verb. The core verbs (`list`, `show`, `create`, `edit`,
`delete`) mean the same thing on every noun.

```sh
cinc node list
cinc node list --profile staging --format json
cinc version
```

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
- `docs/` — command taxonomy and internal architecture design docs.
- `test/` — acceptance tests, gated behind the `acceptance` build tag.

See [`CLAUDE.md`](CLAUDE.md) for conventions followed when developing
with Claude Code.
