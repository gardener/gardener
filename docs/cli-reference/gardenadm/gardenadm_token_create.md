## gardenadm token create

Create a bootstrap token on the cluster for joining a node

### Synopsis

The [token] is the bootstrap token to be created on the cluster.
This token is used for securely authenticating nodes or clients to the cluster.
It must follow the format "[a-z0-9]{6}.[a-z0-9]{16}" to ensure compatibility with Kubernetes bootstrap token requirements.
If no [token] is provided, gardenadm will automatically generate a secure random token for you.

```
gardenadm token create [token] [flags]
```

### Examples

```
# Create a bootstrap token with a specific ID and secret
gardenadm token create foo123.bar4567890baz123

# Create a bootstrap token with a specific ID and secret and directly print the gardenadm join command
gardenadm token create foo123.bar4567890baz123 --print-join-command

# Generate a random bootstrap token for joining a node
gardenadm token create
```

### Options

```
  -d, --description string                  Description for the bootstrap token (default "Used for joining nodes via `gardenadm join`")
  -h, --help                                help for create
  -j, --print-join-command gardenadm join   Instead of only printing the token, print the full machine-readable gardenadm join command that can be copied and ran on a machine that should join the cluster
  -v, --validity duration                   Validity duration of the bootstrap token (default 1h0m0s)
  -w, --worker-pool-name string             Name of the worker pool to use for the join command. If not provided, it is defaulted to 'worker'. (default "worker")
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm token](gardenadm_token.md)	 - Manage bootstrap and discovery tokens for gardenadm join

