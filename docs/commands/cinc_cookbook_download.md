## cinc cookbook download

Download a cookbook version from the server

```
cinc cookbook download <name> [version] [flags]
```

### Examples

Download a cookbook's latest version into ./<name>-<version>/.

```
cinc cookbook download nginx
```

Download a specific version into a chosen directory.

```
cinc cookbook download nginx 1.2.0 --dir ./cookbooks
```

### Options

```
  -d, --dir string   parent directory to download the cookbook into (default ".")
  -h, --help         help for download
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc cookbook](cinc_cookbook.md)	 - Manage cookbooks on the Cinc Server

