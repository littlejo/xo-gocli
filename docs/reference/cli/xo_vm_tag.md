## xo vm tag

Manage the tags of a virtual machine

### Synopsis

Add or remove the tags of a virtual machine. Tags are free-form labels; filter on
them with --query, for example:

  xo vm list --query "[?contains(tags, 'production')].name_label"

```
xo vm tag [flags]
```

### Examples

```
  xo vm tag add <id> production
  xo vm tag remove <id> production
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

* [xo vm](xo_vm.md)	 - Manage virtual machines
* [xo vm tag add](xo_vm_tag_add.md)	 - Add a tag to a virtual machine
* [xo vm tag remove](xo_vm_tag_remove.md)	 - Remove a tag from a virtual machine

