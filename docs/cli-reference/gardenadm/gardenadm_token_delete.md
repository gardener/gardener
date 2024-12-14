## gardenadm token delete

Delete a bootstrap token on the server

### Synopsis

This command will delete a bootstrap token for you.The [token-id] is the ID of the token of the form "[a-z0-9]{6}" to delete

```
gardenadm token delete [token-id] [flags]
```

### Examples

```
# Delete a bootstrap token with id "foo123" on the server
gardenadm token delete foo123
```

### Options

```
  -h, --help   help for delete
```

### SEE ALSO

* [gardenadm token](gardenadm_token.md)	 - Manage bootstrap and discovery tokens for gardenadm join

