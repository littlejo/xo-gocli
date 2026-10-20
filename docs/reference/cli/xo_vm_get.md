## xo vm get

Get a virtual machine

### Synopsis

Get a virtual machine from Xen Orchestra, as a detailed, human
readable view.

The VM is referenced by its UUID, as returned by 'xo vm list'.

The VM can also be given by its exact name (name_label), at the cost of
one more request; a name shared by several VMs is an error that lists
their ids.

The human (default) view is a detail sheet: identity, location (the
host or pool the VM runs on, shown by name), resources and configuration.
The relationships are resolved to names by a small number of extra lookups
(the container and the template); if one cannot be resolved, the raw id is
shown instead and the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo vm get <id> --output json' is unchanged.

```
xo vm get <id|name> [flags]
```

### Examples

```
  xo vm get 550e8400-e29b-41d4-a716-446655440001
  xo vm get web-01
  xo vm get <id> --output json
  xo vm get <id> --query 'name_label'
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

* [xo vm](xo_vm.md)	 - Manage virtual machines

