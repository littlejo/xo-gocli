## xo network get

Get a network

### Synopsis

Get a network from Xen Orchestra, as a detailed, human readable view.

The network is referenced by its UUID, as returned by 'xo network list'.

The network can also be given by its exact name (name_label), at the cost of
one more request; a name shared by several networks is an error that lists
their ids.

The human (default) view is a detail sheet: the bridge, MTU and type, the pool
it belongs to (shown by name), and the counts of VIFs and PIFs. The pool
relationship is resolved by a single lookup; if it cannot be resolved, the raw
id is shown instead and the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo network get <id> --output json' is unchanged.

```
xo network get <id|name> [flags]
```

### Examples

```
  xo network get 11111111-1111-4111-8111-111111111111
  xo network get lan
  xo network get <id> --output json
  xo network get <id> --query 'name_label'
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

* [xo network](xo_network.md)	 - Manage networks

