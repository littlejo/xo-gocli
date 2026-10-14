## xo vm suspend

Suspend a virtual machine

### Synopsis

Suspend a virtual machine.

Suspending saves the VM's memory to its storage and releases it: the VM is
in "Suspended" state until resumed with 'xo vm resume <id>'. The VM must be
running. This is the memory-preserving alternative to 'xo vm stop':
suspending keeps the guest's state exactly, whereas stopping shuts it down.

The VM is referenced by its UUID, as returned by 'xo vm list'.

```
xo vm suspend <id> [flags]
```

### Examples

```
  xo vm suspend 550e8400-e29b-41d4-a716-446655440001
```

### Options

```
  -h, --help   help for suspend
      --wait   wait for the task to complete before returning, and print it (like 'xo task wait'; or set XOA_WAIT=1)
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

