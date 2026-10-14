## xo network list

List networks

### Synopsis

List networks from Xen Orchestra.

```
xo network list [flags]
```

### Examples

```
  xo network list
  xo network list --output json
  xo network list --query '[].name_label'
  xo network list --limit 10
```

### Options

```
  -h, --help           help for list
      --limit int      maximum number of networks to return (0 for no limit)
  -q, --query string   JMESPath expression applied to the result, e.g. '[].name_label'
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

* [xo network](xo_network.md)	 - Manage networks

