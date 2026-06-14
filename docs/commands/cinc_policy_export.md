## cinc policy export

Assemble a standalone bundle from a Policyfile lock

```
cinc policy export [lock] [dir] [flags]
```

### Options

```
  -a, --archive   also write the bundle as a .tar.gz archive
  -h, --help      help for export
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc policy](cinc_policy.md)	 - Manage Policyfile policies on the Cinc Server

