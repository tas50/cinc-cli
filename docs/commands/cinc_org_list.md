## cinc org list

List organizations on the server

### Synopsis

List every organization on the server.

This hits the server root, so it needs a pivotal (superuser) identity.

```
cinc org list [flags]
```

### Examples

List every organization on the server.

```bash
cinc org list
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc org](cinc_org.md)	 - Manage organizations on the Cinc Server

