## xo pool get

Get a pool

### Synopsis

Get a pool from Xen Orchestra, as a detailed, human readable view.

The pool is referenced by its UUID, as returned by 'xo pool list'.

The pool can also be given by its exact name (name_label), at the cost of
one more request; a name shared by several pools is an error that lists
their ids.

The human (default) view is a detail sheet that resolves the master host and
the storage repositories (default, suspend and crash-dump) by name, and lists
the pool's hosts by name (resolved in one batch, never one lookup per host).
The platform version, CPU topology, HA and the pool features are shown too.
If a reference cannot be resolved, the raw id is shown instead and the
command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo pool get <id> --output json' is unchanged.

```
xo pool get <id|name> [flags]
```

### Examples

```
  xo pool get 550e8400-e29b-41d4-a716-446655440001
  xo pool get pool-prod
  xo pool get <id> --output json
  xo pool get <id> --query 'name_label'
```

### Options

```
  -h, --help           help for get
  -q, --query string   JMESPath expression applied to the result, e.g. 'name_label'
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

