## xo vdi import

Import a raw or VHD image into a virtual disk (VDI)

### Synopsis

Import a raw (default) or VHD image into an existing virtual disk (VDI).

The image is read from <file>, or from stdin when <file> is "-". The VDI is
referenced by its UUID, as returned by 'xo vdi list', and its content is
OVERWRITTEN by the imported image:

  xo vdi import <id> disk.raw
  cat disk.raw | xo vdi import <id> -
  xo vdi import <id> disk.vhd --format vhd

This is a destructive operation and asks for confirmation unless --yes is
given.

The upload is a single HTTP request, so the global HTTP client timeout
(--timeout, default 30s) bounds the whole transfer: a large disk over a slow
link can exceed it. Raise it with --timeout or $XOA_TIMEOUT when importing
large disks.

```
xo vdi import <id> <file> [flags]
```

### Examples

```
  xo vdi import 11111111-1111-4111-8111-111111111111 disk.raw
  xo vdi import <id> - --format vhd --yes < disk.vhd
```

### Options

```
      --format string   image format: raw or vhd (default "raw")
  -h, --help            help for import
      --yes             do not ask for confirmation (or set XOA_YES=1)
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

