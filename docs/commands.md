# cinc commands

This page documents every command currently in `cinc` and the flags each
accepts. For background on the overall command grammar see the design
docs under [`dev/`](dev/).

## Synopsis

```
cinc [global flags] <noun> <verb> [args] [flags]
```

Commands are noun-verb. The core verbs (`list`, `create`, `delete`) mean
the same thing on every noun.

## Global flags

Every command accepts the following persistent flags:

| Flag | Description |
| --- | --- |
| `--config <path>` | Path to the credentials file. Default `~/.cinc/credentials`. |
| `--profile <name>` | Profile to read from the credentials file. Default: `$CINC_PROFILE`, then `$CHEF_PROFILE`, then `default`. |
| `--format <human\|json>` | Output format. Default `human`. JSON output is intended for scripting; human is the default for interactive use. |

## `cinc version`

Prints the binary's version, commit, and build date (injected at build
time via `-ldflags`). Takes no arguments or flags.

```sh
cinc version
```

## `cinc node`

Manage nodes on the Cinc/Chef Server.

### `cinc node list`

List every node name on the server, sorted alphabetically.

```sh
cinc node list
cinc node list --format json
```

### `cinc node delete <name>`

Delete a node by name. Prints `Deleted node "<name>"` on success.

```sh
cinc node delete web01
```

## `cinc client`

Manage API clients on the Cinc/Chef Server.

### `cinc client list`

List every API client on the server.

```sh
cinc client list
```

### `cinc client create <name>`

Create an API client. By default the server generates an RSA key pair
and the private key is streamed to stdout so it can be piped to a file.

```sh
cinc client create worker-01 > worker-01.pem
```

| Flag | Description |
| --- | --- |
| `--validator` | Create the client as a validator client (used to bootstrap new nodes). |
| `-f`, `--key-file <path>` | Write the generated private key to this file (mode `0600`) instead of stdout. A confirmation line is printed to stdout. |
| `--public-key <path>` | Send an existing PEM public key. The server will not generate a key pair; no private key is returned. |

### `cinc client delete <name>`

Delete an API client by name. Prints `Deleted client "<name>"` on
success.

```sh
cinc client delete worker-01
```

## `cinc role`

Manage roles on the Cinc/Chef Server.

### `cinc role list`

List every role on the server.

```sh
cinc role list
```

### `cinc role delete <name>`

Delete a role by name. Prints `Deleted role "<name>"` on success.

```sh
cinc role delete web
```

## `cinc environment`

Manage environments on the Cinc/Chef Server.

### `cinc environment list`

List every environment on the server.

```sh
cinc environment list
```

### `cinc environment delete <name>`

Delete an environment by name. Prints `Deleted environment "<name>"` on
success.

```sh
cinc environment delete staging
```

## `cinc cookbook`

Manage cookbooks on the Cinc/Chef Server.

### `cinc cookbook list`

List every cookbook on the server (names only; version metadata is not
shown).

```sh
cinc cookbook list
```

### `cinc cookbook delete <name> <version>`

Delete a single cookbook version. Both name and version are required —
the server identifies a cookbook by both. Prints
`Deleted cookbook "<name>" version <version>` on success.

```sh
cinc cookbook delete nginx 1.0.0
```

## `cinc data-bag`

Manage data bags on the Cinc/Chef Server.

### `cinc data-bag list`

List every data bag on the server.

```sh
cinc data-bag list
```

### `cinc data-bag delete <name>`

Delete a data bag by name. Prints `Deleted data bag "<name>"` on
success.

```sh
cinc data-bag delete users
```
