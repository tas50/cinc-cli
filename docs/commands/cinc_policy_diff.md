## cinc policy diff

Compare two revisions of a policy

### Synopsis

Compare two revisions of a policy.

By default ref1 and ref2 name policy groups and the comparison is
between the revision active in each. Pass --revisions to treat them as
revision ids instead: cinc policy diff NAME --revisions A B.

```
cinc policy diff <name> <ref1> <ref2> [flags]
```

### Examples

Compare two revisions of a policy.

```
cinc policy diff appserver 1.0.0 1.1.0
```

### Options

```
  -h, --help        help for diff
      --revisions   treat the two refs as revision ids rather than policy group names
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc policy](cinc_policy.md)	 - Manage Policyfile policies on the Cinc Server

