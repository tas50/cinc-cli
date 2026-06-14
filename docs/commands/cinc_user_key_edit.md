## cinc user key edit

Edit one of a user's keys

```
cinc user key edit <user> <key-name> [flags]
```

### Examples

Edit one of a user's keys, for example its expiration.

```bash
cinc user key edit alice default
```

### Options

```
      --file string   read the updated key JSON from this file instead of launching the editor
  -h, --help          help for edit
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc user key](cinc_user_key.md)	 - Manage a user's public keys

