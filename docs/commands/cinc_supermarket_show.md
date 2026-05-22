## cinc supermarket show

Show a cookbook (or one of its versions) on Chef Supermarket

### Synopsis

Without a version argument, shows the cookbook record: maintainer,
description, latest version, total downloads, and the versions
published. With a version argument, shows that version's license,
tarball size, dependencies, and supported platforms.

```
cinc supermarket show <cookbook> [version] [flags]
```

### Options

```
  -h, --help                      help for show
      --supermarket-site string   URL of the Chef Supermarket site (default: https://supermarket.chef.io)
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc supermarket](cinc_supermarket.md)	 - Manage cookbooks on Chef Supermarket

