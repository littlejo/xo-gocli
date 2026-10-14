## xo pbd unplug

Unplug a PBD (disconnect the SR from its host)

### Synopsis

Unplug a PBD, disconnecting its storage repository (SR) from the host it
belongs to. Use 'xo pbd plug <id>' to re-connect it.

The PBD is referenced by its UUID, as returned by 'xo pbd list'. The action
is asynchronous: the command prints the task id and returns. Track it with
'xo task get <task-id>' or 'xo task wait <task-id>'.

```
xo pbd unplug <id> [flags]
```

### Examples

```
  xo pbd unplug 550e8400-e29b-41d4-a716-446655440001
  xo pbd unplug <id> --output json
```

### Options

```
  -h, --help   help for unplug
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

