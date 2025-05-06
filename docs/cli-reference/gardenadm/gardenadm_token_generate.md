## gardenadm token generate

Generate a random bootstrap token for joining a node

### Synopsis

Generate a random bootstrap token that can be used for joining a node to an autonomous shoot cluster.
Note that the token is not created on the server (use 'gardenadm token create' for it).
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
  -h, --help   help for generate
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm token](gardenadm_token.md)	 - Manage bootstrap and discovery tokens for gardenadm join

