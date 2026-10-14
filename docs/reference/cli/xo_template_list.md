## xo template list

List VM templates

### Synopsis

List VM templates from Xen Orchestra.

Templates are first-class objects in Xen Orchestra (REST resource
"vm-templates"); unlike ordinary VMs they are not part of the "vms"
collection, which is why 'xo vm list' does not show them.

```
xo template list [flags]
```

### Examples

```
  xo template list
  xo template list --output json
  xo template list --query '[].name_label'
  xo template list --limit 10
```

### Options

```
  -h, --help           help for list
      --limit int      maximum number of templates to return (0 for no limit)
  -q, --query string   JMESPath expression applied to the result, e.g. '[].name_label'
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

