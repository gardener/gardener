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
```

### Options

```
  -d, --description string                     Description for the bootstrap token (default "Used for joining nodes via `gardenadm join`")
  -h, --help                                   help for generate
  -v, --validity duration                      Validity duration of the bootstrap token (default 1h0m0s)
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm token](gardenadm_token.md)	 - Manage bootstrap and discovery tokens for gardenadm join

