## gardenadm token create

Create a bootstrap token on the server

### Synopsis

The [token] is the actual token to write.This should be a securely generated random token of the form "[a-z0-9]{6}.[a-z0-9]{16}".If no [token] is given, gardenadm will generate a random token instead.

```
gardenadm token create [token] [flags]
```

### Examples

```
# Create a bootstrap token with id "foo123" on the server
gardenadm token create foo123.bar4567890baz123

# Create a bootstrap token generated randomly
gardenadm token create
```

### Options

```
  -h, --help   help for create
```

### SEE ALSO

* [gardenadm token](gardenadm_token.md)	 - Manage bootstrap and discovery tokens for gardenadm join

