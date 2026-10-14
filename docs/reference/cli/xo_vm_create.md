## xo vm create

Create a virtual machine in a pool

### Synopsis

Create a virtual machine in a pool from a template.

The pool is referenced by its UUID, as returned by 'xo pool list'. The
template accepts the id printed by 'xo template list' (the composite
<poolId>-<templateUuid> id) or the bare template UUID (the 'uuid' field of
'xo template list --output json'); both refer to the same template. The
template must belong to the given pool.

Memory, when given, is expressed in bytes or as a human readable size
(e.g. 2G, 512M). If omitted, the template default is used.

Use --boot to start the VM as soon as it has been created.

The creation is asynchronous server-side; this command waits for the backing
task to finish before printing the created VM. The wait has no deadline: it
ends when the task completes, or on Ctrl+C.

```
xo vm create <name> [flags]
```

### Examples

```
  xo vm create web-02 --pool <pool-id> --template <template-id>
  xo vm create web-02 --pool <pool-id> --template <template-id> --memory 4G --description "web server"
  xo vm create web-02 --pool <pool-id> --template <template-id> --boot
```

### Options

```
      --boot                 start the VM as soon as it has been created
      --description string   description for the VM
  -h, --help                 help for create
      --memory string        memory size, in bytes or human readable (e.g. 2G, 512M)
      --pool string          pool UUID to create the VM in (see 'xo pool list')
      --template string      template id to base the VM on: the bare template UUID or the composite id printed by 'xo template list'
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

