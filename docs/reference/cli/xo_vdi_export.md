## xo vdi export

Export a virtual disk (VDI) to a raw or VHD image

### Synopsis

Export a virtual disk (VDI) to a raw (default) or VHD image.

The image is streamed to stdout unless --file is given (use "-" for an
explicit stdout), so it can be piped:

  xo vdi export <id> > disk.raw
  xo vdi export <id> --file disk.raw

The VDI is referenced by its UUID, as returned by 'xo vdi list'.

The image is streamed over a single HTTP request, so the global HTTP client
timeout (--timeout, default 30s) bounds the whole transfer: a large disk over
a slow link can exceed it. Raise it with --timeout or $XOA_TIMEOUT when
exporting or importing large disks.

```
xo vdi export <id> [flags]
```

### Examples

```
  xo vdi export 11111111-1111-4111-8111-111111111111 --file disk.raw
  xo vdi export <id> > disk.raw
  xo vdi export <id> --format vhd --file disk.vhd
```

### Options

```
      --file string     destination file (default: stdout; use - to be explicit)
      --format string   image format: raw or vhd (default "raw")
  -h, --help            help for export
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

