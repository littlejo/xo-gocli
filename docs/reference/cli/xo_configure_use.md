## xo configure use

Make a profile the current one

### Synopsis

Make a stored profile the current one: the profile every command uses when
neither --profile nor $XOA_PROFILE is set. 'xo configure list' marks it.

```
xo configure use <name> [flags]
```

### Examples

```
  xo configure use lab
  xo configure list
```

### Options

```
  -h, --help   help for use
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

