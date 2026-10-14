## xo vbd list

List virtual block devices (VBDs)

### Synopsis

List virtual block devices (VBDs) from Xen Orchestra.

A VBD is the attachment point between a VM and a VDI (virtual disk).

```
xo vbd list [flags]
```

### Examples

```
  xo vbd list
  xo vbd list --vm <vm-id>
  xo vbd list --output json
  xo vbd list --query '[].VDI'
  xo vbd list --limit 10
```

### Options

```
  -h, --help           help for list
      --limit int      maximum number of VBDs to return (0 for no limit)
  -q, --query string   JMESPath expression applied to the result, e.g. '[].VDI'
      --vm string      only list the VBDs of the given VM (UUID)
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

