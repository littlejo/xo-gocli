## xo token

Manage authentication tokens

### Synopsis

List, inspect and create the authentication tokens of the current user.
A token is the recommended way to authenticate: store it with
'xo configure --token'.

The token id is the secret: it is printed in full once, by 'create', and
masked everywhere else.

```
xo token [flags]
```

### Examples

```
  xo token create --description "ci" --expires-in "30 days"
  xo token list
```

### Options

```
  -h, --help   help for token
```

### Options inherited from parent commands

```
  -d, --debug              show SDK/API error details on failure (or $XOA_DEBUG)
      --json               output as JSON (shorthand for --output json)
  -o, --output string      output format: table, json, yaml, text (or $XOA_DEFAULT_OUTPUT, or the profile's "output" value) (default "table")
  -p, --profile string     name of the configuration profile to use (or $XOA_PROFILE)
      --timeout duration   HTTP client timeout, e.g. 60s or 2m (default 30s, or $XOA_TIMEOUT)
```

### SEE ALSO

* [xo](xo.md)	 - Xen Orchestra command line client
* [xo token create](xo_token_create.md)	 - Create an authentication token
* [xo token get](xo_token_get.md)	 - Get an authentication token
* [xo token list](xo_token_list.md)	 - List authentication tokens

