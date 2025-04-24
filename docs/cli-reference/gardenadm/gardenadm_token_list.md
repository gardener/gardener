## gardenadm token list

Display a list of all bootstrap tokens available on the server.

### Synopsis

The "list" command retrieves and displays all bootstrap tokens currently available on the server. 
Bootstrap tokens are used for authenticating nodes during the join process.

```
gardenadm token list [flags]
```

### Examples

```
# To list all bootstrap tokens available on the server:
gardenadm token list

# To include additional sensitive details such as token secrets:
gardenadm token list --with-token-secret
```

### Options

```
  -h, --help                help for list
      --with-token-secret   Display the token secret
```

### Options inherited from parent commands

```
      --log-format string   The format for the logs. Must be one of [json text] (default "text")
      --log-level string    The level/severity for the logs. Must be one of [debug info error] (default "info")
```

### SEE ALSO

* [gardenadm token](gardenadm_token.md)	 - Manage bootstrap and discovery tokens for gardenadm join

