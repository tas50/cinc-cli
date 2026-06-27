# Migrating from Chef

Already have a working knife or Chef Workstation setup? You're most of
the way to running `cinc`. This guide gets you productive fast, focusing
on the part that matters most: your credentials. For the exhaustive
configuration reference, see [Configuring cinc](configuration.md); for
exact command flags, see the [per-command reference](commands/).

## The short version

`cinc` reads a TOML credentials file in the **same shape** as Chef's
`~/.chef/credentials`. The keys are the same (`chef_server_url`,
`client_name`, `client_key`, …), so your existing file already speaks
the language `cinc` understands. You have two ways to reuse it:

- **Migrate it once.** The first time you run a `cinc` command without a
  `~/.cinc/credentials` file, `cinc` offers to copy your existing Chef
  credentials over for you.
- **Point at it directly.** Run any command with
  `--config ~/.chef/credentials` to use your Chef file in place, no copy
  required.

The rest of this page covers both, plus the handful of config details
worth knowing as you settle in.

## Reusing your existing credentials

### First-run migration

`cinc` reads `~/.cinc/credentials` by default. When that file is
**missing** and you run a command interactively (a real terminal, no
explicit `--config`), `cinc` notices and offers to set you up. If it
finds a `~/.chef/credentials` file, it asks whether to migrate it:

```text
We found an existing Chef config at /home/tim/.chef/credentials.
Want us to migrate it to /home/tim/.cinc/credentials for you? [Y/n]
```

Say yes and `cinc` reads every profile from your Chef file and writes
the equivalent `~/.cinc/credentials`. It carries over each profile's
server URL, `client_name`, `client_key`, `ssl_verify_mode`, and
`supermarket_site`. Your `~/.chef/credentials` is left untouched.

A few things to be precise about, because the migration copies the
profile fields it knows and nothing else:

- It's **a one-time copy, not a live fallback.** After migration, `cinc`
  reads `~/.cinc/credentials`; it does not transparently fall back to
  `~/.chef/credentials` on later runs. Changes you make to the Chef file
  afterward won't show up in `cinc` unless you migrate again or edit the
  cinc file.
- The migration prompt only appears in an **interactive terminal** when
  the default cinc file is missing and you didn't pass `--config`. In a
  script or CI (no TTY), nothing is migrated automatically — set the
  file up ahead of time, or point `--config` at an existing one.
- `secret_file` and the cinc-only `supermarket_client_name` /
  `supermarket_key` keys are **not** copied by migration. If you relied
  on them, re-add them to `~/.cinc/credentials` afterward (it's plain
  TOML), or just keep pointing `--config` at your original file.

If there's no Chef file to migrate, the first-run flow drops into the
same interactive setup as [`cinc config create`](configuration.md#cinc-config-create).

### Pointing at your Chef file directly

Because the file formats match, you can skip copying entirely and use
your Chef credentials in place:

```sh
cinc node list --config ~/.chef/credentials
```

This is handy when you want one canonical credentials file, or you're
trying `cinc` out without committing to a new file yet.

## Chef- and cinc-prefixed keys

`cinc` is cinc-first but chef-compatible. **Every** chef-prefixed config
key and environment variable has a cinc-prefixed equivalent, and both
are accepted. When both are set in the same profile or environment, the
`cinc_`/`CINC_` form wins. That means your existing Chef keys keep
working untouched, and you can override individual settings with the
cinc form without rewriting the whole file.

| Chef-prefixed | Cinc-prefixed | Where |
| --- | --- | --- |
| `chef_server_url` | `cinc_server_url` | TOML key |
| `CHEF_PROFILE` | `CINC_PROFILE` | env var |
| `CHEF_SECRET_FILE` | `CINC_SECRET_FILE` | env var |

Most credential keys — `client_name`, `client_key`, `ssl_verify_mode`,
`secret_file` — are spelled the same in both worlds; only the ones above
have a distinct chef/cinc spelling. As with knife, the server URL must
include the `/organizations/<org>` segment.

## Config changes worth knowing

### Supermarket uploads use your client identity by default

This is the one most likely to trip up an existing user. `cinc
supermarket share` signs uploads with your profile's `client_name` and
`client_key` — the same identity it uses against your Chef server. But
the **public** Supermarket (`https://supermarket.chef.io`) usually wants
a *different* identity: your Supermarket account, not your Chef client.

Two cinc-only keys let you override the upload identity while still
talking to your Chef server with your normal client:

```toml
[default]
cinc_server_url         = "https://cinc.example.com/organizations/acme"
client_name             = "tim"
client_key              = "/keys/tim.pem"
supermarket_client_name = "tim-public"
supermarket_key         = "/keys/supermarket.pem"
```

Each falls back independently — set just the username, just the key, or
both — and when neither is set, uploads use `client_name`/`client_key`.
See [the configuration reference](configuration.md#supermarket_site-and-the-upload-identity)
for the full story.

### Encrypted data bag secrets

The `secret_file` key points at your encrypted data bag secret, exactly
as knife's `knife[:secret_file]` does, so an existing
`encrypted_data_bag_secret` works unchanged. You can also override it
per command with `--secret-file`/`--secret`, or via `$CINC_SECRET_FILE`
(chef-compat: `$CHEF_SECRET_FILE`). Full resolution order is in the
[configuration reference](configuration.md#secret_file-and-encrypted-data-bags).

### SSL verification

`ssl_verify_mode` works the same as in knife: `:verify_peer` (the
default) verifies the server certificate, `:verify_none` skips it for a
lab server with a self-signed cert.

## Command mapping

`cinc` is **noun-verb** — `cinc <noun> <verb>` — and the core verbs
(`list`, `show`, `create`, `edit`, `delete`) mean the same thing on
every noun. Most knife commands map naturally:

| knife | cinc |
| --- | --- |
| `knife node show web01` | `cinc node show web01` |
| `knife node list` | `cinc node list` |
| `knife data bag create secrets` | `cinc databag create secrets` |
| `knife cookbook upload nginx` | `cinc cookbook upload nginx` |
| `knife supermarket share my_cookbook` | `cinc supermarket share my_cookbook` |
| `knife ssl check` | `cinc config validate` (validates config and pings the server) |

This is **not a 1:1 translation, nor a definitive list** — the grammar
is more regular than knife's, some commands take different flags, and a
few knife subcommands have no cinc equivalent (and vice versa). For the
exact verbs, flags, and behavior of any command, always check the
[per-command reference](commands/), which is generated from the live
command tree and can't drift from the binary.

## Where to go next

- [Configuring cinc](configuration.md) — the definitive reference for
  every credentials key, profile selection, and `cinc config`.
- [Using cinc](README.md) — a guided tour of the noun-verb grammar,
  output formats, and common workflows.
- [Command reference](commands/) — every command, subcommand, and flag.
