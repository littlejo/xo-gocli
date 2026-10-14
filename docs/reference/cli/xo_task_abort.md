## xo task abort

Abort a running task

### Synopsis

Abort a task that is still running.

Xen Orchestra requests the interruption of the operation backing the task
(a VM start, a migration, a provisioning, ...). The task then reaches the
"interrupted" status; the operation itself may take a little time to unwind,
and it is not always fully reversible (for example, a VM start aborted
mid-way leaves the VM in the state it reached).

Only a task that is still pending can be aborted: aborting a task that
already reached a terminal state (success, failure or interrupted) is
rejected with a clear error.

This is a destructive operation and asks for confirmation unless --yes is
given. The task is referenced by its id, as returned by 'xo task list' or by
an asynchronous operation (for example 'xo vm start').

```
xo task abort <id> [flags]
```

### Examples

```
  xo task abort a1b2c3d4e5f6
  xo task abort <id> --yes
  xo task wait <id> --wait-timeout 5m   # then abort if it is stuck
```

### Options

```
  -h, --help   help for abort
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

* [xo task](xo_task.md)	 - Manage asynchronous tasks

