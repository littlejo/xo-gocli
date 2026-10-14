## xo vdi tag

Manage the tags of a virtual disk (VDI)

### Synopsis

Add or remove the tags of a virtual disk (VDI). Tags are free-form labels; filter on
them with --query, for example:

  xo vdi list --query "[?contains(tags, 'production')].name_label"

```
xo vdi tag [flags]
```

### Examples

```
  xo vdi tag add <id> production
  xo vdi tag remove <id> production
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

* [xo vdi](xo_vdi.md)	 - Manage virtual disks (VDIs)
* [xo vdi tag add](xo_vdi_tag_add.md)	 - Add a tag to a virtual disk (VDI)
* [xo vdi tag remove](xo_vdi_tag_remove.md)	 - Remove a tag from a virtual disk (VDI)

