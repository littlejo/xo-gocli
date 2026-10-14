## xo vbd disconnect

Hot-unplug a VBD (detach the disk from a running VM)

### Synopsis

Hot-unplug a VBD, dynamically detaching its VDI from the running VM.

The VBD is referenced by its UUID, as returned by 'xo vbd list'. Use
'xo vbd delete <id>' instead to remove the attachment entirely (keeping the
VDI).

```
xo vbd disconnect <id> [flags]
```

### Examples

```
  xo vbd disconnect 33333333-3333-4333-8333-333333333333
```

### Options

```
  -h, --help   help for disconnect
      --wait   wait for the task to complete before returning, and print it (like 'xo task wait'; or set XOA_WAIT=1)
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

