## xo vdi list

List virtual disks (VDIs)

### Synopsis

List virtual disks (VDIs) from Xen Orchestra.

A VDI is a virtual disk that lives on a storage repository (SR). Use
'xo vm vdis <id>' to list only the disks attached to a specific VM.

```
xo vdi list [flags]
```

### Examples

```
  xo vdi list
  xo vdi list --output json
  xo vdi list --query '[].name_label'
  xo vdi list --type user
  xo vdi list --limit 10
```

### Options

```
  -h, --help           help for list
      --limit int      maximum number of VDIs to return (0 for no limit)
  -q, --query string   JMESPath expression applied to the result, e.g. '[].name_label'
      --type string    filter by VDI type: user, system, suspend, …
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

