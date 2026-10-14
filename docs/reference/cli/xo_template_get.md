## xo template get

Get a VM template

### Synopsis

Get a VM template from Xen Orchestra, as a detailed, human readable view.

The template is referenced by its id, as returned by 'xo template list'.

The human (default) view is a detail sheet: the memory and CPUs, the pool the
template belongs to (shown by name), and the power state. If the pool cannot
be resolved, the raw id is shown instead and the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo template get <id> --output json' is
unchanged.

```
xo template get <id> [flags]
```

### Examples

```
  xo template get d31e47fd-a70e-d849-883e-c17193472710-6959dfe8-534c-4c58-8a8c-3c3792293543
  xo template get <id> --output json
  xo template get <id> --query 'name_label'
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

* [xo template](xo_template.md)	 - Manage VM templates

