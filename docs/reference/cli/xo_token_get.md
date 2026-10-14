## xo token get

Get an authentication token

### Synopsis

Get an authentication token of the current user.

The token is referenced by its id (the full secret value, as returned by
'xo token create' or 'xo token list --no-secret'). The REST API does not
expose a per-token endpoint, so this is resolved against the token list.

The token id is the secret, so it is masked in the output by default;
pass --no-secret to reveal the full value (use with care).

```
xo token get <id> [flags]
```

### Examples

```
  xo token get <id>
  xo token get <id> --output json
  xo token get <id> --query 'description'
```

### Options

```
  -h, --help           help for get
      --no-secret      do not mask the token secret in the output
  -q, --query string   JMESPath expression applied to the result, e.g. 'description'
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

* [xo token](xo_token.md)	 - Manage authentication tokens

