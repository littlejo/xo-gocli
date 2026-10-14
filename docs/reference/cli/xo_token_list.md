## xo token list

List authentication tokens

### Synopsis

List the authentication tokens of the current user.

Tokens are what the CLI itself uses to authenticate (see 'xo configure').
The token id is the secret, so it is masked in the output by default;
pass --no-secret to reveal the full values (use with care).

```
xo token list [flags]
```

### Examples

```
  xo token list
  xo token list --output json
  xo token list --query '[].description'
  xo token list --no-secret --query '[].id'
```

### Options

```
  -h, --help           help for list
      --limit int      maximum number of tokens to return (0 for no limit)
      --no-secret      do not mask the token secret in the output
  -q, --query string   JMESPath expression applied to the result, e.g. '[].description'
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

