## cinc node ssh

Run an SSH command on nodes

```
cinc node ssh [search-query] [command] [flags]
```

### Options

```
      --attribute string           node attribute used as the SSH host (default "fqdn")
      --concurrency int            maximum concurrent SSH sessions (default 10)
      --exit-on-error              stop launching new SSH sessions after the first failure
  -h, --help                       help for ssh
      --host-key-verify            verify SSH host keys using known_hosts (default true)
      --no-client                  treat search query as a space-separated host list and skip Cinc Server lookup
      --no-host-key-verify         disable SSH host key verification
      --ssh-agent                  use SSH agent authentication (default true)
      --ssh-agent-socket string    SSH agent socket path
      --ssh-identity-file string   SSH identity file
      --ssh-password string        SSH password
      --ssh-port int               SSH port (default 22)
      --ssh-timeout duration       SSH connection timeout (default 30s)
      --ssh-user string            SSH user
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc node](cinc_node.md)	 - Manage nodes on the Cinc Server

