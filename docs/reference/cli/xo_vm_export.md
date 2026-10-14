## xo vm export

Export a virtual machine to an XVA or OVA archive

### Synopsis

Export a virtual machine to an XVA (default) or OVA archive.

The archive is streamed to stdout unless --file is given (use "-" for an
explicit stdout), so it can be piped:

  xo vm export <id> > vm.xva
  xo vm export <id> --file vm.xva

The VM is referenced by its UUID, as returned by 'xo vm list'.

This operation is not exposed by the SDK v2 typed service yet, so it is
performed through the SDK's own REST client against GET /vms/<id>.<format>
(documented in xva.go; to contribute upstream).

The archive is streamed over a single HTTP request, so the global HTTP
client timeout (--timeout, default 30s) bounds the whole transfer: a large
XVA or OVA over a slow link can exceed it. Raise it with --timeout or
$XOA_TIMEOUT when exporting or importing large archives.

```
xo vm export <id> [flags]
```

### Examples

```
  xo vm export 550e8400-e29b-41d4-a716-446655440001 --file web-01.xva
  xo vm export <id> > web-01.xva
  xo vm export <id> --format ova --file web-01.ova
  xo vm export <id> --compress=false
```

### Options

```
      --compress        compress the archive (xva only) (default true)
      --file string     destination file (default: stdout; use - to be explicit)
      --format string   archive format: xva or ova (default "xva")
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

* [xo vm](xo_vm.md)	 - Manage virtual machines

