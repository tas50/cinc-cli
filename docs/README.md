# Using cinc

This directory holds the user-facing documentation for the `cinc`
command-line tool. The top-level project [`README`](../README.md)
covers install and a short overview; this page goes into how to
actually use `cinc` day-to-day.

## Getting started

Three steps to your first successful command:

**1. Install the binary.** Tagged releases ship prebuilt Linux and
macOS archives at <https://github.com/tas50/cinc-cli/releases>.
Download the archive for your platform, extract it, and drop
`cinc` somewhere on your `PATH`:

```sh
curl -fsSL -o cinc.tar.gz \
  https://github.com/tas50/cinc-cli/releases/latest/download/cinc_linux_amd64.tar.gz
tar -xzf cinc.tar.gz
sudo install cinc_*/cinc /usr/local/bin/cinc
cinc version
```

If you have a Go toolchain handy, `make build` (or `make install`)
from a checkout works too.

**2. Point cinc at your server.** Already have a Chef
`~/.chef/credentials`? See [Migrating from Chef](migrating-from-chef.md)
— `cinc` can migrate it for you on first run, or read it in place with
`--config ~/.chef/credentials`. Otherwise, run the interactive
configurator:

```sh
cinc config create
```

It prompts for the server URL (including the `/organizations/<org>`
segment), the client name, and the path to the client's PEM private
key, then writes `~/.cinc/credentials`. Pass `--profile staging`
(or any name) to set up additional profiles next to the default. For
the full credentials reference, see
[Configuring cinc](configuration.md).

**3. Verify and use it.** Sanity-check the profile, then run your
first command:

```sh
cinc config validate     # parse the file and ping the server
cinc node list           # list nodes on the configured server
```

If `validate` reports a problem, `cinc config create` will let
you fix it. From here, every other command follows the same
noun-verb grammar described below.

## Where to find what

- **[`configuration.md`](configuration.md)** — the definitive
  configuration reference: every credentials key, multiple profiles,
  profile selection, encrypted data bag secrets, Supermarket upload
  identities, and the `cinc config` commands.
- **[`migrating-from-chef.md`](migrating-from-chef.md)** — for knife and
  Chef Workstation users: reusing your existing `~/.chef/credentials`,
  the chef/cinc key duality, and how knife commands map onto cinc.
- **[`commands/`](commands/)** — auto-generated reference for every
  command and flag, regenerated from the live cobra command tree on
  every push to `main`. Start at [`commands/cinc.md`](commands/cinc.md)
  for the root command, or jump straight to a resource:
  - [`cinc client`](commands/cinc_client.md) — API clients
    (`create`, `delete`, `edit`, `list`, `show`)
  - [`cinc config`](commands/cinc_config.md) — local configuration
    (`create`, `validate`)
  - [`cinc cookbook`](commands/cinc_cookbook.md) — cookbooks
    (`delete`, `list`, `show`, `upload`)
  - [`cinc databag`](commands/cinc_databag.md) — data bags
    (`create`, `delete`, `list`, `show`, `item delete`, `item edit`,
    `item list`, `item show`)
  - [`cinc environment`](commands/cinc_environment.md) — environments
    (`create`, `delete`, `edit`, `list`, `show`)
  - [`cinc explore`](commands/cinc_explore.md) — a k9s-style terminal UI
    for the whole server (browse, view, edit, create, delete, download)
  - [`cinc group`](commands/cinc_group.md) — ACL groups
    (`create`, `delete`, `edit`, `list`, `member add`, `member remove`, `show`)
  - [`cinc node`](commands/cinc_node.md) — nodes
    (`bootstrap`, `delete`, `list`, `show`, `ssh`)
  - [`cinc policy`](commands/cinc_policy.md) — Policyfile policies
    (`clean`, `create`, `delete`, `diff`, `list`, `show`)
  - [`cinc policy-group`](commands/cinc_policy-group.md) — policy groups
    (`delete`, `list`, `show`)
  - [`cinc role`](commands/cinc_role.md) — roles
    (`create`, `delete`, `edit`, `list`, `show`)
  - [`cinc search`](commands/cinc_search.md) — search the server for
    nodes, roles, environments, clients, or data bag items
  - [`cinc supermarket`](commands/cinc_supermarket.md) — cookbooks on
    Chef Supermarket (`download`, `explore`, `list`, `search`, `share`,
    `show`)
  - [`cinc user`](commands/cinc_user.md) — global users
    (`create`, `delete`, `edit`, `list`, `password`, `show`)
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
