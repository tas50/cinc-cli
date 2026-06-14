## cinc client key create

Add a key to a client

```
cinc client key create <client> <key-name> [flags]
```

### Options

```
      --expires string      expiration date (ISO-8601 UTC) or 'infinity' (default "infinity")
  -h, --help                help for create
  -f, --key-file string     write the generated private key to this file instead of stdout
      --public-key string   path to a PEM public key; the server will not generate a key pair
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc client key](cinc_client_key.md)	 - Manage a client's public keys

