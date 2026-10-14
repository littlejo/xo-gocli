## xo host tag add

Add a tag to a host

### Synopsis

Add a tag to a host.

The host is referenced by its UUID, as returned by 'xo host list'.

```
xo host tag add <id> <tag> [flags]
```

### Examples

```
  xo host tag add aaaaaaaa-bbbb-cccc-dddd-000000000001 production
  xo host tag add <id> web
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

* [xo host tag](xo_host_tag.md)	 - Manage the tags of a host

