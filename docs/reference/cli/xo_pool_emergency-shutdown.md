## xo pool emergency-shutdown

Shut down every host of the pool immediately

### Synopsis

Shut down every host of a pool, without moving the VMs away first.

Unlike a rolling reboot, nothing is evacuated: every host (and every VM
running on it) is powered off at once. The pool is down afterwards and the
hosts must be restarted manually to recover. Use it only when the pool is in
a bad state and cannot be handled by any other means.

This is a destructive operation and asks for confirmation unless --yes is
given. The command is synchronous: it blocks until the backing task completes
(or Ctrl+C) and reports the final result. There is no default wait deadline,
so use --wait-timeout to bound the wait if you need one. The global --timeout
still bounds each HTTP request (each poll), not the whole wait.

The pool is referenced by its UUID, as returned by 'xo pool list'.

```
xo pool emergency-shutdown <id> [flags]
```

### Examples

```
  xo pool emergency-shutdown aaaaaaaa-bbbb-cccc-dddd-000000000001
  xo pool emergency-shutdown <id> --yes
  xo pool emergency-shutdown <id> --yes --wait-timeout 10m
```

### Options

```
  -h, --help                    help for emergency-shutdown
      --wait-timeout duration   wait at most this long for the operation to complete, e.g. 30m (default: wait until it completes or Ctrl+C)
      --yes                     do not ask for confirmation (or set XOA_YES=1)
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

* [xo pool](xo_pool.md)	 - Manage pools

