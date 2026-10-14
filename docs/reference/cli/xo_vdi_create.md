## xo vdi create

Create a virtual disk (VDI) on a storage repository

### Synopsis

Create a virtual disk (VDI) on a storage repository (SR).

The SR is referenced by its UUID, as returned by 'xo sr list'. The size,
when given, is expressed in bytes or as a human readable size (e.g. 2G,
512M).

Creating a VDI only allocates the disk on the SR; it is not attached to any
VM yet. Attach it with 'xo vbd create --vm <vm-id> --vdi <id>' (see
docs/usecases.md, 'Add a disk to a VM').

```
xo vdi create <name> [flags]
```

### Examples

```
  xo vdi create data-01 --sr <sr-id> --size 10G
  xo vdi create data-01 --sr <sr-id> --size 10G --description "data disk"
  xo vdi create shared-disk --sr <sr-id> --size 1G --shared --tags common,scratch
```

### Options

```
      --description string   description for the VDI
  -h, --help                 help for create
      --shared               allow the VDI to be attached to several VMs at once
      --size string          virtual size, in bytes or human readable (e.g. 2G, 512M) (required)
      --sr string            SR UUID to create the VDI on (required, see 'xo sr list')
      --tags string          comma-separated tags for the VDI
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

