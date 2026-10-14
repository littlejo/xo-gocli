## xo network create

Create a network in a pool

### Synopsis

Create a network in a pool, attached to a PIF (physical interface) of
one of the pool's hosts.

The pool is referenced by its UUID, as returned by 'xo pool list'. The PIF
is referenced by its UUID as well; PIFs can be listed with 'xo rest get pifs'
(there is no typed PIF command yet).

The creation is asynchronous server-side; this command waits for the backing
task to finish before printing the created network. The wait has no
deadline: it ends when the task completes, or on Ctrl+C.

```
xo network create <name> [flags]
```

### Examples

```
  xo network create web --pool <pool-id> --pif <pif-id>
  xo network create vlan-100 --pool <pool-id> --pif <pif-id> --vlan 100
  xo network create mgmt --pool <pool-id> --pif <pif-id> --mtu 9000
```

### Options

```
      --description string   description for the network
  -h, --help                 help for create
      --mtu int              network MTU (0 for the XenServer default, 1500)
      --nbd                  allow NBD access to the network
      --pif string           PIF (physical interface) UUID to attach the network to (required, see 'xo rest get pifs')
      --pool string          pool UUID to create the network in (required, see 'xo pool list')
      --vlan uint            VLAN tag, between 0 and 4094 (0 for an untagged network)
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

