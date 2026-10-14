## xo task

Manage asynchronous tasks

### Synopsis

List and inspect the asynchronous tasks Xen Orchestra runs (VM actions,
migrations, imports, ...), wait for one to complete, or abort it.

'xo task wait' exits 0 only when the task succeeds, so it can gate a
script.

Tasks are referenced by their id, as printed by 'xo task list' or by an
asynchronous command.

```
xo task [flags]
```

### Examples

```
  xo task list --status pending
  xo task wait <id> --wait-timeout 10m
  xo task abort <id>
```

### Options

```
  -h, --help   help for task
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

* [xo](xo.md)	 - Xen Orchestra command line client
* [xo task abort](xo_task_abort.md)	 - Abort a running task
* [xo task get](xo_task_get.md)	 - Get a task
* [xo task list](xo_task_list.md)	 - List asynchronous tasks
* [xo task wait](xo_task_wait.md)	 - Wait for a task to complete

