## xo task wait

Wait for a task to complete

### Synopsis

Wait for a task to reach a terminal state (success, failure or
interrupted), polling every 2 seconds, and print it like 'xo task get'.

The command blocks until the task completes. Ctrl+C cancels the wait. Use
--wait-timeout to bound how long to wait; without it the wait is unbounded (it ends
only when the task completes or is interrupted).

Exit status reflects the task's outcome, so it is safe to use as a gate in
scripts:
  - 0 when the task completes successfully
  - non-zero when the task fails, is interrupted, the --wait-timeout deadline
    is reached, or the task does not exist
The completed task is always printed (in the chosen --output format); a
non-zero exit is reported on stderr.

The global --timeout (or $XOA_TIMEOUT) still bounds each HTTP request, that
is each poll, not the whole wait.

The task is referenced by its id, as returned by 'xo task list' or by an
asynchronous operation (for example 'xo vm start').

```
xo task wait <id> [flags]
```

### Examples

```
  xo task wait a1b2c3d4e5f6
  xo task wait <id> --wait-timeout 5m
  xo task wait <id> --output json
  xo task wait <id> --query 'status'
```

### Options

```
  -h, --help                    help for wait
  -q, --query string            JMESPath expression applied to the completed task, e.g. 'status'
      --wait-timeout duration   wait at most this long for the task to complete, e.g. 2m (default: wait until it completes)
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

