## cinc configure

Create or update a local credentials profile

```
cinc configure [flags]
```

### Options

```
      --chef-server-url string    Cinc Server URL including /organizations/<org>
      --cinc-server-url string    Cinc Server URL including /organizations/<org>
      --client-key string         path to the PEM private key for the client
      --client-name string        client name used to sign API requests
  -h, --help                      help for configure
      --server-url string         Cinc Server URL including /organizations/<org>
      --skip-key-check            write the profile without checking that --client-key exists
      --ssl-verify-mode string    optional SSL verify mode such as :verify_peer or :verify_none
      --supermarket-site string   Chef Supermarket URL for cookbook uploads
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc](cinc.md)	 - Cinc is a unified command-line tool for Cinc/Chef Infra

