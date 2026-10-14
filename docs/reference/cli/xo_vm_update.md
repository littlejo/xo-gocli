## xo vm update

Update the name, description or tags of a virtual machine

### Synopsis

Update the name_label, name_description and/or tags of a virtual machine.

At least one of --name, --description or --tags is required. Only the fields
that are given are changed; the others are left untouched.

--tags REPLACES the full tag list of the VM (it is not a merge). Give the
complete, comma-separated list you want the VM to end up with. To add or
remove a single tag without touching the others, use 'xo vm tag add/remove'.

The VM is referenced by its UUID, as returned by 'xo vm list'.

Note: this operation is not exposed by the SDK v2 typed service yet, so it is
performed through the SDK's own REST client against PATCH /vms/<id>.

```
xo vm update <id> [flags]
```

### Examples

```
  xo vm update 550e8400-e29b-41d4-a716-446655440001 --name web-01
  xo vm update <id> --description "primary web server"
  xo vm update <id> --tags production,web
  xo vm update <id> --name web-01 --description "primary web server" --tags prod
```

### Options

```
      --description string   new name_description for the VM
  -h, --help                 help for update
      --name string          new name_label for the VM
      --tags string          full comma-separated list of tags to set (replaces the existing list)
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

