## xo sr tag

Manage the tags of a storage repository

### Synopsis

Add or remove the tags of a storage repository. Tags are free-form labels; filter on
them with --query, for example:

  xo sr list --query "[?contains(tags, 'production')].name_label"

```
xo sr tag [flags]
```

### Examples

```
  xo sr tag add <id> production
  xo sr tag remove <id> production
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

* [xo sr](xo_sr.md)	 - Manage storage repositories
* [xo sr tag add](xo_sr_tag_add.md)	 - Add a tag to a storage repository
* [xo sr tag remove](xo_sr_tag_remove.md)	 - Remove a tag from a storage repository

