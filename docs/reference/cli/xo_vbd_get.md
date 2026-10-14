## xo vbd get

Get a virtual block device (VBD)

### Synopsis

Get a virtual block device (VBD) from Xen Orchestra, as a detailed,
human readable view.

A VBD is the attachment point between a VM and a VDI (virtual disk). The
VBD is referenced by its UUID, as returned by 'xo vbd list'.

The human (default) view is a detail sheet that resolves the relationships
by name: the VM the VBD belongs to, and the VDI it points at (with its
size). If a reference cannot be resolved, the raw id is shown instead and
the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo vbd get <id> --output json' is unchanged.

```
xo vbd get <id> [flags]
```

### Examples

```
  xo vbd get 33333333-3333-4333-8333-333333333333
  xo vbd get <id> --output json
  xo vbd get <id> --query 'VDI'
```

### Options

```
  -h, --help           help for get
  -q, --query string   JMESPath expression applied to the result, e.g. 'VDI'
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

* [xo vbd](xo_vbd.md)	 - Manage virtual block devices (VBDs)

