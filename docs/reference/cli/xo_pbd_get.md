## xo pbd get

Get a physical block device (PBD)

### Synopsis

Get a physical block device (PBD) from Xen Orchestra, as a detailed,
human readable view.

A PBD is the connection between a host and a storage repository (SR); it
is what "plugs" an SR into a host. The PBD is referenced by its UUID, as
returned by 'xo pbd list'.

The human (default) view is a detail sheet that resolves the relationships
by name (the host, the SR and the pool) and shows the full device_config,
not just the block device (for example the server/serverpath of an NFS SR).
If a reference cannot be resolved, the raw id is shown instead and the
command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo pbd get <id> --output json' is unchanged.

```
xo pbd get <id> [flags]
```

### Examples

```
  xo pbd get 550e8400-e29b-41d4-a716-446655440001
  xo pbd get <id> --output json
  xo pbd get <id> --query 'device_config'
```

### Options

```
  -h, --help           help for get
  -q, --query string   JMESPath expression applied to the result, e.g. 'device_config'
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

