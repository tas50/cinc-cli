## cinc node bootstrap

Bootstrap a node with Cinc Client over SSH

```
cinc node bootstrap [target] [flags]
```

### Options

```
      --bootstrap-url string       Cinc install script URL (default "https://omnitruck.cinc.sh/install.sh")
      --bootstrap-version string   Cinc Client version to install
      --dry-run                    print the bootstrap script without creating a client or connecting
      --environment string         first run environment (default "_default")
  -h, --help                       help for bootstrap
      --host-key-verify            verify SSH host keys using known_hosts (default true)
      --no-host-key-verify         disable SSH host key verification
      --node-name string           node name to configure on the Cinc Server (default target host)
      --policy-group string        policy group for first run
      --policy-name string         policyfile name for first run
      --run-list string            comma-separated first run-list
      --ssh-agent                  use SSH agent authentication (default true)
      --ssh-agent-socket string    SSH agent socket path
      --ssh-identity-file string   SSH identity file
      --ssh-password string        SSH password
      --ssh-port int               SSH port (default 22)
      --ssh-timeout duration       SSH connection timeout (default 30s)
      --ssh-user string            SSH user
      --sudo                       run bootstrap commands through sudo (default true)
```

### Options inherited from parent commands

```
      --config string    path to the cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc node](cinc_node.md)	 - Manage nodes on the Cinc/Chef Server

