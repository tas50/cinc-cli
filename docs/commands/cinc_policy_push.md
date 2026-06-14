## cinc policy push

Deploy a Policyfile lock to a policy group

```
cinc policy push <group> [lock] [flags]
```

### Examples

Deploy ./Policyfile.lock.json to a policy group, uploading its cookbooks.

```
cinc policy push prod
```

Deploy a specific lock file.

```
cinc policy push prod Policyfile.lock.json
```

### Options

```
  -h, --help   help for push
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc policy](cinc_policy.md)	 - Manage Policyfile policies on the Cinc Server

