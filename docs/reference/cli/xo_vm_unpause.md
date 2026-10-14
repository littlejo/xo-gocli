## xo vm unpause

Unpause a virtual machine

### Synopsis

Unpause a virtual machine.

The VM's vCPUs resume where 'xo vm pause' stopped them. The VM is
referenced by its UUID, as returned by 'xo vm list'.

```
xo vm unpause <id> [flags]
```

### Examples

```
  xo vm unpause 550e8400-e29b-41d4-a716-446655440001
```

### Options

```
  -h, --help   help for unpause
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

