## xo task get

Get a task

### Synopsis

Get a task from Xen Orchestra, as a detailed, human readable view.

The task is referenced by its id, as returned by 'xo task list' or by an
asynchronous operation (for example 'xo vm start').

The human (default) view is a detail sheet: the status, the operation (type
and name), when it started and ended, its duration, the user that ran it and
the object it targeted. For a failed task the error (code and message) is
shown; for a successful one with a result, that result is shown.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo task get <id> --output json' is unchanged.

```
xo task get <id> [flags]
```

### Examples

```
  xo task get a1b2c3d4e5f6
  xo task get <id> --output json
  xo task get <id> --query 'result'
```

### Options

```
  -h, --help           help for get
  -q, --query string   JMESPath expression applied to the result, e.g. 'result'
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

