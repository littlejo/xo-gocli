## xo sr reclaim-space

Reclaim unused space from a storage repository

### Synopsis

Reclaim unused space from a storage repository.

Space reclamation (thin-provisioning "unmap") returns unused disk space to
the underlying storage. It only applies to SR types that support it (for
example thin-provisioned LVM or VDI-based stores); other SR types will be
rejected by Xen Orchestra. The operation is asynchronous and returns a task
id.

The SR is referenced by its UUID, as returned by 'xo sr list'.

```
xo sr reclaim-space <id> [flags]
```

### Examples

```
  xo sr reclaim-space 550e8400-e29b-41d4-a716-446655440001
```

### Options

```
  -h, --help   help for reclaim-space
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

* [xo sr](xo_sr.md)	 - Manage storage repositories

