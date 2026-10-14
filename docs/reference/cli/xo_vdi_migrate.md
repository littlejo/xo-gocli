## xo vdi migrate

Migrate a virtual disk (VDI) to another storage repository

### Synopsis

Migrate a virtual disk (VDI) to another storage repository (SR).

The VDI is referenced by its UUID, as returned by 'xo vdi list'. The target
SR must belong to the same pool as the VDI (see 'xo sr list').

The operation is asynchronous: Xen Orchestra returns a task and the command
prints it. Track it with 'xo task get <task-id>' or
'xo task wait <task-id>'.

Note: after the migration completes, the VDI has a NEW id. Look it up again
with 'xo vdi list' (filter by name) or 'xo vm vdis <vm-id>' to get the new
reference.

```
xo vdi migrate <id> --sr <sr-id> [flags]
```

### Examples

```
  xo vdi migrate 11111111-1111-4111-8111-111111111111 --sr aaaaaaaa-bbbb-cccc-dddd-000000000002
  xo vdi migrate <id> --sr <sr-id> --output json
```

### Options

```
  -h, --help        help for migrate
      --sr string   target storage repository UUID (required; see 'xo sr list')
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

