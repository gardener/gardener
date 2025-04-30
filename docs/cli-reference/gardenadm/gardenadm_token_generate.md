## gardenadm token generate

Generate a random bootstrap token for joining a node

### Synopsis

Generate a random bootstrap token that can be used for joining a node to an autonomous shoot cluster. 
The token is securely generated and follows the format "[a-z0-9]{6}.[a-z0-9]{16}".
Read more about it here: https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/

```
gardenadm token generate [flags]
```

### Examples

```
# Generate a random bootstrap token for joining a node
gardenadm token generate

# Generate a random bootstrap token for joining a node and secret and directly print the gardenadm join command
gardenadm token generate --print-join-command
```

### Options

```
  -d, --description string                  Description for the bootstrap token (default "Used for joining nodes via `gardenadm join`")
  -h, --help                                help for generate
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

