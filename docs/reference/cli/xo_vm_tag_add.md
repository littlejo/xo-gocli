## xo vm tag add

Add a tag to a virtual machine

### Synopsis

Add a tag to a virtual machine.

The VM is referenced by its UUID, as returned by 'xo vm list'.

```
xo vm tag add <id> <tag> [flags]
```

### Examples

```
  xo vm tag add 550e8400-e29b-41d4-a716-446655440001 production
  xo vm tag add <id> web
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

* [xo vm tag](xo_vm_tag.md)	 - Manage the tags of a virtual machine

