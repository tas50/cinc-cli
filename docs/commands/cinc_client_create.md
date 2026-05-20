## cinc client create

Create an API client on the server

```
cinc client create <name> [flags]
```

### Options

```
  -h, --help                help for create
  -f, --key-file string     write the generated private key to this file instead of stdout
      --public-key string   path to a PEM public key; the server will not generate a key pair
      --validator           create a validator client
```

### Options inherited from parent commands

```
      --config string    path to the cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc client](cinc_client.md)	 - Manage API clients on the Cinc/Chef Server

