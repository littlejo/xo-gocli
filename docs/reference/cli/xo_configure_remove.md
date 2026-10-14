## xo configure remove

Remove a configuration profile

### Synopsis

Remove a stored configuration profile.

This is a destructive operation and asks for confirmation unless --yes is
given (or $XOA_YES is set). When the removed profile was the current one,
current falls back to the first remaining profile.

```
xo configure remove <name> [flags]
```

### Examples

```
  xo configure remove lab
  xo configure remove lab --yes
```

### Options

```
  -h, --help   help for remove
      --yes    do not ask for confirmation (or set XOA_YES=1)
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

