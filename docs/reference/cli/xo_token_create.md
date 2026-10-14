## xo token create

Create an authentication token

### Synopsis

Create an authentication token for the current user.

The created token is printed once in full (its id is the secret). Save it
now: it can be listed later but only masked, unless you kept the value.

If --client-id is given and a non-expired token already exists for that
client id, the existing token is updated and returned instead of creating a
new one.

```
xo token create [flags]
```

### Examples

```
  xo token create
  xo token create --description "ci pipeline" --expires-in "30 days"
  xo token create --client-id my-cli --expires-in "1 hour"
```

### Options

```
      --client-id string     client identifier (reuses the existing token for this client if any)
      --description string   description for the token
      --expires-in string    token lifetime, e.g. "1 hour", "30 days" (default set by the server)
  -h, --help                 help for create
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

