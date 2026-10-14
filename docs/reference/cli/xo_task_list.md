## xo task list

List asynchronous tasks

### Synopsis

List asynchronous tasks from Xen Orchestra.

Tasks are returned by most asynchronous operations (VM actions,
provisioning, ...). Each task has a status: pending, success, failure or
interrupted.

```
xo task list [flags]
```

### Examples

```
  xo task list
  xo task list --output json
  xo task list --query '[].id'
  xo task list --status failure
  xo task list --limit 10
```

### Options

```
  -h, --help            help for list
      --limit int       maximum number of tasks to return (0 for no limit)
  -q, --query string    JMESPath expression applied to the result, e.g. '[].id'
      --status string   filter by status: pending, success, failure, interrupted
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

