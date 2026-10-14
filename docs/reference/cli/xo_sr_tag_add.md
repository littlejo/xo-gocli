## xo sr tag add

Add a tag to a storage repository

### Synopsis

Add a tag to a storage repository.

The SR is referenced by its UUID, as returned by 'xo sr list'.

```
xo sr tag add <id> <tag> [flags]
```

### Examples

```
  xo sr tag add 11111111-1111-4111-8111-111111111111 production
  xo sr tag add <id> web
```

### Options

```
  -h, --help   help for add
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

* [xo sr tag](xo_sr_tag.md)	 - Manage the tags of a storage repository

