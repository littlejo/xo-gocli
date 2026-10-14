## xo pool rolling-update

Apply the latest pool update, rolling through the hosts

### Synopsis

Apply the latest available pool update.

The update (new Xen Orchestra agent / tools versions) is rolled across the
hosts of the pool one by one, so the pool stays available the whole time.
Each host is rebooted in turn with the VMs that run on it moved away first
(live migration when possible, otherwise a scheduled downtime for the VMs).

The command is synchronous: it blocks until the backing task completes (or
Ctrl+C), and reports the final result. There is no default wait deadline —
a pool update can legitimately run for a long time — so use --wait-timeout
to bound the wait if you need one. The global --timeout still bounds each
HTTP request (each poll), not the whole wait.

The pool is referenced by its UUID, as returned by 'xo pool list'.

```
xo pool rolling-update <id> [flags]
```

### Examples

```
  xo pool rolling-update aaaaaaaa-bbbb-cccc-dddd-000000000001
  xo pool rolling-update <id> --wait-timeout 30m
  xo pool rolling-update <id> --output json
```

### Options

```
  -h, --help                    help for rolling-update
      --wait-timeout duration   wait at most this long for the operation to complete, e.g. 30m (default: wait until it completes or Ctrl+C)
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

