## xo vm delete

Delete a virtual machine

### Synopsis

Delete a virtual machine.

This removes the VM from Xen Orchestra and frees the disks and memory
associated with it. The operation is irreversible: the VM's data is lost.
A running VM must be stopped or suspended first (the API refuses to delete
it otherwise).

This is a destructive operation and asks for confirmation unless --yes is
given. The VM is referenced by its UUID, as returned by 'xo vm list'.

```
xo vm delete <id> [flags]
```

### Examples

```
  xo vm delete 550e8400-e29b-41d4-a716-446655440001
  xo vm delete <id> --yes
```

### Options

```
  -h, --help   help for delete
      --yes    do not ask for confirmation (or set XOA_YES=1)
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

