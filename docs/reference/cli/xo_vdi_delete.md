## xo vdi delete

Delete a virtual disk (VDI)

### Synopsis

Delete a virtual disk (VDI).

This removes the VDI and the data it contains from the storage repository.
The operation is irreversible. The VDI must not be attached to any VM:
detach it first with 'xo vbd delete <vbd-id>' (the API refuses to delete an
attached VDI otherwise).

This is a destructive operation and asks for confirmation unless --yes is
given. The VDI is referenced by its UUID, as returned by 'xo vdi list'.

```
xo vdi delete <id> [flags]
```

### Examples

```
  xo vdi delete 11111111-1111-4111-8111-111111111111
  xo vdi delete <id> --yes
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

* [xo vdi](xo_vdi.md)	 - Manage virtual disks (VDIs)

