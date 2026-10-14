## xo host tag

Manage the tags of a host

### Synopsis

Add or remove the tags of a host. Tags are free-form labels; filter on
them with --query, for example:

  xo host list --query "[?contains(tags, 'production')].name_label"

```
xo host tag [flags]
```

### Examples

```
  xo host tag add <id> production
  xo host tag remove <id> production
```

### Options

```
  -h, --help   help for tag
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

* [xo host](xo_host.md)	 - Manage hosts
* [xo host tag add](xo_host_tag_add.md)	 - Add a tag to a host
* [xo host tag remove](xo_host_tag_remove.md)	 - Remove a tag from a host

