## xo pbd list

List physical block devices (PBDs)

### Synopsis

List physical block devices (PBDs) from Xen Orchestra.

A PBD is the connection between a host and a storage repository (SR); it is
what "plugs" an SR into a host. Use 'xo pbd plug' / 'xo pbd unplug' to
connect or disconnect it.

```
xo pbd list [flags]
```

### Examples

```
  xo pbd list
  xo pbd list --output json
  xo pbd list --query '[].attached'
  xo pbd list --limit 10
```

### Options

```
  -h, --help           help for list
      --limit int      maximum number of PBDs to return (0 for no limit)
  -q, --query string   JMESPath expression applied to the result, e.g. '[].attached'
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

* [xo pbd](xo_pbd.md)	 - Manage physical block devices (PBDs)

