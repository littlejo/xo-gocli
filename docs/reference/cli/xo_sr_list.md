## xo sr list

List storage repositories

### Synopsis

List storage repositories (SRs) from Xen Orchestra.

```
xo sr list [flags]
```

### Examples

```
  xo sr list
  xo sr list --output json
  xo sr list --query '[].name_label'
  xo sr list --type lvm
  xo sr list --limit 10
```

### Options

```
  -h, --help           help for list
      --limit int      maximum number of SRs to return (0 for no limit)
  -q, --query string   JMESPath expression applied to the result, e.g. '[].name_label'
      --type string    filter by SR type (XO live-filter: case-insensitive substring, so 'lvm' also matches 'lvmoiscsi')
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

