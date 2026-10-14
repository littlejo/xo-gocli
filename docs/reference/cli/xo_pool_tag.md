## xo pool tag

Manage the tags of a pool

### Synopsis

Add or remove the tags of a pool. Tags are free-form labels; filter on
them with --query, for example:

  xo pool list --query "[?contains(tags, 'production')].name_label"

```
xo pool tag [flags]
```

### Examples

```
  xo pool tag add <id> production
  xo pool tag remove <id> production
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

* [xo pool](xo_pool.md)	 - Manage pools
* [xo pool tag add](xo_pool_tag_add.md)	 - Add a tag to a pool
* [xo pool tag remove](xo_pool_tag_remove.md)	 - Remove a tag from a pool

