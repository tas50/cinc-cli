## cinc client reregister

Regenerate a client's default key, invalidating the old one

```
cinc client reregister <name> [flags]
```

### Options

```
  -h, --help              help for reregister
  -f, --key-file string   write the new private key to this file instead of stdout
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc client](cinc_client.md)	 - Manage API clients on the Cinc Server

