# Using cinc

This directory holds the user-facing documentation for the `cinc`
command-line tool. The top-level project [`README`](../README.md)
covers install and a short overview; this page goes into how to
actually use `cinc` day-to-day.

## Where to find what

- **[`commands/`](commands/)** — auto-generated reference for every
  command and flag, regenerated from the live cobra command tree on
  every push to `main`. Start at [`commands/cinc.md`](commands/cinc.md)
  for the root command, or jump straight to a resource:
  - [`cinc client`](commands/cinc_client.md) — API clients
  - [`cinc cookbook`](commands/cinc_cookbook.md) — cookbooks
  - [`cinc databag`](commands/cinc_databag.md) — data bags
  - [`cinc environment`](commands/cinc_environment.md) — environments
  - [`cinc node`](commands/cinc_node.md) — nodes
  - [`cinc role`](commands/cinc_role.md) — roles
  - [`cinc version`](commands/cinc_version.md) — version info
- **[`dev/`](dev/)** — design background: the command taxonomy and
  internal architecture. Read these when changing the shape of the
  CLI, not when learning to use it.

## The noun-verb grammar

Every server interaction in `cinc` has the same shape:

```
cinc <noun> <verb> [args] [flags]
```

The **noun** is the resource type (`node`, `role`, `cookbook`, …) and
the **verb** is the action you want to take on it. The core verbs —
`list`, `create`, `delete` (and eventually `show`, `edit`) — mean the
same thing on every noun, so learning one resource teaches you the
others:

```sh
cinc node list
cinc role list
cinc cookbook list
```

all return the names of that resource type on the server, sorted.

A handful of commands take additional arguments or flags that don't
generalize — `cinc client create` returns a freshly generated private
key, `cinc cookbook delete` requires both a name and a version because
the server identifies a cookbook by both. Those quirks are documented
on each command's reference page under [`commands/`](commands/).

## Talking to multiple servers with profiles

`cinc` reads credentials from a TOML file (default
`~/.cinc/credentials`; override with `--config`). Each top-level
section in that file is a **profile**, a named bundle of server URL,
client identity, and signing key:

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

Pick a profile per command with `--profile`, or set `CINC_PROFILE`
(falling back to `CHEF_PROFILE`) to make a choice sticky for the
shell:

```sh
cinc node list                     # uses [default]
cinc node list --profile staging   # uses [staging]
CINC_PROFILE=staging cinc role list
```

`cinc` accepts the Chef-prefixed forms (`chef_server_url`,
`CHEF_PROFILE`) as well, so an existing `~/.chef/credentials` works
unchanged. When both prefixes are set on the same key, the
cinc-prefixed value wins.

## Output formats

Every command honors `--format`. `human` (the default) is meant for
terminals and pipelines into common UNIX tools; `json` is the
machine-readable form.

```sh
$ cinc node list
db01
web01
web02

$ cinc node list --format json
[
  "db01",
  "web01",
  "web02"
]
```

Use `json` to drive further processing (`jq` and friends); leave
the flag off when you're driving the CLI interactively.

## Common workflows

### Inspecting what's on the server

```sh
cinc node list
cinc role list
cinc environment list
cinc client list
cinc cookbook list
cinc databag list
```

### Provisioning a new client identity

```sh
cinc client create worker-01 --key-file ./worker-01.pem
```

The server generates the RSA key pair; `cinc` writes the private key
to the file you name (mode `0600`) and prints a confirmation. With no
`--key-file`, the key streams to stdout instead — handy for piping:

```sh
cinc client create worker-02 > worker-02.pem
```

If you already have a public key and want the server to use that
instead of generating one, point at it with `--public-key`.

### Tearing things down

```sh
cinc node delete web02
cinc role delete web
cinc cookbook delete nginx 1.0.0
```

`cookbook delete` requires both a name and a version because the
server identifies a cookbook by both.

## Need more detail?

Each command's exhaustive reference — every flag, every default, every
inherited option — is in [`commands/`](commands/). The pages there are
the source of truth; this overview is intentionally selective.
