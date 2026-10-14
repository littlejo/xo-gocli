## xo vm import

Import a virtual machine from an XVA archive

### Synopsis

Import a virtual machine from an XVA archive into a pool.

The XVA file is read from <file>, or from stdin when <file> is "-". The
--pool flag is required and selects the pool the VM is imported into
(see 'xo pool list'); --sr optionally pins the VM's disks to a specific
storage repository of that pool.

This operation is not exposed by the SDK v2 typed service yet, so it is
performed through the SDK's own REST client against POST /pools/<pool>/vms
(documented in xva.go; to contribute upstream).

The upload is a single HTTP request, so the global HTTP client timeout
(--timeout, default 30s) bounds the whole transfer: a large XVA over a slow
link can exceed it. Raise it with --timeout or $XOA_TIMEOUT when importing
large archives.

```
xo vm import <file> [flags]
```

### Examples

```
  xo vm import web-01.xva --pool 6b7c8d9e-0000-1111-2222-333344445555
  xo vm import web-01.xva --pool <pool-id> --sr <sr-id>
  cat web-01.xva | xo vm import - --pool <pool-id>
```

### Options

```
  -h, --help          help for import
      --pool string   pool UUID to import the VM into (see 'xo pool list')
      --sr string     SR UUID of the pool to pin the VM's disks to (default: pool default)
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

