## cinc environment create

Create an environment on the server

```
cinc environment create <name> [flags]
```

### Examples

Create an environment; your editor opens to define it.

```
cinc environment create prod
```

### Options

```
  -d, --description string   human-readable description for the new environment
      --file string          read the full environment JSON from this file instead of using flags
  -h, --help                 help for create
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc environment](cinc_environment.md)	 - Manage environments on the Cinc Server

