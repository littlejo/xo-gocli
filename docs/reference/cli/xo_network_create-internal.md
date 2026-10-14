## xo network create-internal

Create an internal network in a pool

### Synopsis

Create an internal network in a pool.

An internal network does not attach to a physical interface; it carries
virtual traffic between VMs within the pool (for example a management or a
dedicated VM-to-VM network).

The pool is referenced by its UUID, as returned by 'xo pool list'. The
creation is asynchronous server-side; this command waits for the backing
task to finish before printing the created network. The wait has no
deadline: it ends when the task completes, or on Ctrl+C.

```
xo network create-internal <name> [flags]
```

### Examples

```
  xo network create-internal internal --pool <pool-id>
  xo network create-internal internal --pool <pool-id> --mtu 9000
```

### Options

```
      --description string   description for the network
  -h, --help                 help for create-internal
      --mtu int              network MTU (0 for the XenServer default, 1500)
      --nbd                  allow NBD access to the network
      --pool string          pool UUID to create the network in (required, see 'xo pool list')
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

* [xo network](xo_network.md)	 - Manage networks

