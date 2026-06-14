## cinc node show

Show a node

```
cinc node show <name> [flags]
```

### Examples

Show a node's full object, including its attributes and run-list.

```
cinc node show web01
```

### Options

```
  -h, --help   help for show
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc node](cinc_node.md)	 - Manage nodes on the Cinc Server

