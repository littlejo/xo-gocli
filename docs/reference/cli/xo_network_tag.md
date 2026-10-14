## xo network tag

Manage the tags of a network

### Synopsis

Add or remove the tags of a network. Tags are free-form labels; filter on
them with --query, for example:

  xo network list --query "[?contains(tags, 'production')].name_label"

```
xo network tag [flags]
```

### Examples

```
  xo network tag add <id> production
  xo network tag remove <id> production
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

* [xo network](xo_network.md)	 - Manage networks
* [xo network tag add](xo_network_tag_add.md)	 - Add a tag to a network
* [xo network tag remove](xo_network_tag_remove.md)	 - Remove a tag from a network

