## xo vm pause

Pause a virtual machine

### Synopsis

Pause a virtual machine.

Pausing suspends the execution of all the VM's vCPUs: the VM keeps its RAM
contents and its power state reads "Paused" while paused. Resume it with
'xo vm unpause <id>'. Pausing is not the same as suspending: a suspended VM
is saved to disk and its memory is released (see 'xo vm suspend').

The VM is referenced by its UUID, as returned by 'xo vm list'.

```
xo vm pause <id> [flags]
```

### Examples

```
  xo vm pause 550e8400-e29b-41d4-a716-446655440001
```

### Options

```
  -h, --help   help for pause
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

