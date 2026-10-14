## xo template

Manage VM templates

### Synopsis

List and inspect the VM templates available in each pool. A template id
printed here is what 'xo vm create --template' takes.

```
xo template [flags]
```

### Examples

```
  xo template list
  xo template list --query '[].name_label'
  xo template get <id>
```

### Options

```
  -h, --help   help for template
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
* [xo template get](xo_template_get.md)	 - Get a VM template
* [xo template list](xo_template_list.md)	 - List VM templates

