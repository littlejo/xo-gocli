## xo vbd delete

Detach a VDI from a VM (delete the VBD)

### Synopsis

Delete a virtual block device (VBD), detaching the VDI from the VM.

Only the attachment is removed; the VDI (and the data it contains) is kept on
the storage repository. To also delete the disk, run 'xo vdi delete <vdi-id>'
afterwards (the VDI must have no remaining VBD).

This is a destructive operation and asks for confirmation unless --yes is
given. The VBD is referenced by its UUID, as returned by 'xo vbd list'.

```
xo vbd delete <id> [flags]
```

### Examples

```
  xo vbd delete 33333333-3333-4333-8333-333333333333
  xo vbd delete <id> --yes
```

### Options

```
  -h, --help   help for delete
      --yes    do not ask for confirmation (or set XOA_YES=1)
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

