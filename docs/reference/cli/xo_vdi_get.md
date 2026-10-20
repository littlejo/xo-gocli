## xo vdi get

Get a virtual disk (VDI)

### Synopsis

Get a virtual disk (VDI) from Xen Orchestra, as a detailed,
human readable view.

The VDI is referenced by its UUID, as returned by 'xo vdi list' or
'xo vm vdis <id>'.

The VDI can also be given by its exact name (name_label), at the cost of
one more request; a name shared by several VDIs is an error that lists
their ids.

The human (default) view is a detail sheet that resolves the SR by name and
lists the VMs the disk is attached to (resolved in one batch, never one
lookup per VM). The disk's type, size, usage, snapshots and tags are shown
too. If a reference cannot be resolved, the raw id is shown instead and the
command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo vdi get <id> --output json' is unchanged.

```
xo vdi get <id|name> [flags]
```

### Examples

```
  xo vdi get 11111111-1111-4111-8111-111111111111
  xo vdi get data-disk
  xo vdi get <id> --output json
  xo vdi get <id> --query 'name_label'
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

* [xo vdi](xo_vdi.md)	 - Manage virtual disks (VDIs)

