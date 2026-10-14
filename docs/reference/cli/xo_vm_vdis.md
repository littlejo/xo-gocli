## xo vm vdis

List the virtual disks (VDIs) of a virtual machine

### Synopsis

List the virtual disks (VDIs) attached to a virtual machine.

The VM is referenced by its UUID, as returned by 'xo vm list'.

```
xo vm vdis <id> [flags]
```

### Examples

```
  xo vm vdis 550e8400-e29b-41d4-a716-446655440001
  xo vm vdis <id> --output json
  xo vm vdis <id> --query '[].name_label'
  xo vm vdis <id> --type user
  xo vm vdis <id> --limit 10
```

### Options

```
  -h, --help           help for vdis
      --limit int      maximum number of VDIs to return (0 for no limit)
  -q, --query string   JMESPath expression applied to the result, e.g. '[].name_label'
      --type string    filter by VDI type: user, system, suspend, rrd, …
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

* [xo vm](xo_vm.md)	 - Manage virtual machines

