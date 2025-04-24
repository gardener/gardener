## gardenadm token delete

Delete one or more bootstrap tokens on the server

### Synopsis

Delete one or more bootstrap tokens on the server.

The [token-value] is the ID of the token of the form "[a-z0-9]{6}" to delete.
Alternatively, it can be the full token value of the form "[a-z0-9]{6}.[a-z0-9]{16}".
A third option is to specify the name of the Secret in the form "bootstrap-token-[a-z0-9]{6}".

You can delete multiple tokens by providing multiple token values separated by spaces.

```
gardenadm token delete [token-values...] [flags]
```

### Examples

```
# Delete a single bootstrap token with ID "foo123" on the server
gardenadm token delete foo123

# Delete multiple bootstrap tokens with IDs "foo123", "bar456", and "789baz" on the server
gardenadm token delete foo123 bootstrap-token-bar456 789baz.abcdef1234567890

# Attempt to delete a token that does not exist (will not throw an error if the token is already deleted)
gardenadm token delete nonexisting123
```

### Options

```
  -h, --help   help for delete
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm token](gardenadm_token.md)	 - Manage bootstrap and discovery tokens for gardenadm join

