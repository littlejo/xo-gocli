## xo network create-bonded

Create a bonded network in a pool

### Synopsis

Create a bonded network in a pool.

A bonded network links several PIFs (physical interfaces) of the pool hosts
into a single logical network for redundancy or aggregation.

The pool is referenced by its UUID, as returned by 'xo pool list'; the PIFs
by their UUIDs, as listed by 'xo rest get pifs'. --bond-mode selects the
bonding mode: active-backup, balance-slb or lacp.

The creation is asynchronous server-side; this command waits for the backing
task to finish before printing the created network. The wait has no
deadline: it ends when the task completes, or on Ctrl+C.

```
xo network create-bonded <name> [flags]
```

### Examples

```
  xo network create-bonded bond0 --pool <pool-id> --pifs <pif-1>,<pif-2> --bond-mode active-backup
  xo network create-bonded bond0 --pool <pool-id> --pifs <pif-1>,<pif-2> --bond-mode lacp --mtu 9000
```

### Options

```
      --bond-mode string     bonding mode: active-backup, balance-slb or lacp (required)
      --description string   description for the network
  -h, --help                 help for create-bonded
      --mtu int              network MTU (0 for the XenServer default, 1500)
      --nbd                  allow NBD access to the network
      --pifs string          comma-separated PIF UUIDs to bond (required, see 'xo rest get pifs')
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

