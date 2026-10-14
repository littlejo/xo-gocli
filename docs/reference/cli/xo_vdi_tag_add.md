## xo vdi tag add

Add a tag to a virtual disk (VDI)

### Synopsis

Add a tag to a virtual disk (VDI).

The VDI is referenced by its UUID, as returned by 'xo vdi list'.

```
xo vdi tag add <id> <tag> [flags]
```

### Examples

```
  xo vdi tag add 11111111-1111-4111-8111-111111111111 production
  xo vdi tag add <id> web
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

* [xo vdi tag](xo_vdi_tag.md)	 - Manage the tags of a virtual disk (VDI)

