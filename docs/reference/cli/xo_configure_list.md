## xo configure list

List configuration profiles

### Synopsis

List the stored configuration profiles.

Credentials are masked; 'show' reveals a single profile in full.

```
xo configure list [flags]
```

### Examples

```
  xo configure list
  xo configure list --output json
  xo configure list --query '[].name'
```

### Options

```
  -h, --help           help for list
  -q, --query string   JMESPath expression applied to the result, e.g. '[].name'
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

* [xo configure](xo_configure.md)	 - Manage configuration profiles

