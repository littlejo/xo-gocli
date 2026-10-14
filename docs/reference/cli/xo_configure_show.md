## xo configure show

Show a configuration profile

### Synopsis

Show a stored configuration profile in full, including the token or
password. Without a name, the selected profile is shown (--profile or
$XOA_PROFILE; the current profile when neither is set).

```
xo configure show [name] [flags]
```

### Examples

```
  xo configure show
  xo configure show lab
  xo configure show lab --output json
```

### Options

```
  -h, --help           help for show
  -q, --query string   JMESPath expression applied to the result, e.g. 'endpoint'
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

