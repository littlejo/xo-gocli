## xo pbd plug

Plug a PBD (connect the SR to its host)

### Synopsis

Plug a PBD, connecting its storage repository (SR) to the host it belongs
to. Use 'xo pbd unplug <id>' to do the opposite.

The PBD is referenced by its UUID, as returned by 'xo pbd list'. The action
is asynchronous: the command prints the task id and returns. Track it with
'xo task get <task-id>' or 'xo task wait <task-id>'.

```
xo pbd plug <id> [flags]
```

### Examples

```
  xo pbd plug 550e8400-e29b-41d4-a716-446655440001
  xo pbd plug <id> --output json
```

### Options

```
  -h, --help   help for plug
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

* [xo pbd](xo_pbd.md)	 - Manage physical block devices (PBDs)

