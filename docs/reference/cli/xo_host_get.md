## xo host get

Get a host

### Synopsis

Get a host from Xen Orchestra, as a detailed, human readable view.

The host is referenced by its UUID, as returned by 'xo host list'.

The human (default) view is a detail sheet: the address, the pool the host
belongs to (shown by name), the memory usage, the CPU model and the VMs
resident on the host (resolved in one batch, never one lookup per VM).
Platform, license and status fields are shown too. If the pool cannot be
resolved, the raw id is shown instead and the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo host get <id> --output json' is unchanged.

```
xo host get <id> [flags]
```

### Examples

```
  xo host get 550e8400-e29b-41d4-a716-446655440001
  xo host get <id> --output json
  xo host get <id> --query 'name_label'
```

### Options

```
  -h, --help           help for get
  -q, --query string   JMESPath expression applied to the result, e.g. 'name_label'
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

* [xo host](xo_host.md)	 - Manage hosts

