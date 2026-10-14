## xo network delete

Delete a network

### Synopsis

Delete a network.

This removes the network from the pool. The operation is irreversible and
refused while the network still has attached virtual interfaces (VIFs): the
connected VMs must be dealt with first.

This is a destructive operation and asks for confirmation unless --yes is
given. The network is referenced by its UUID, as returned by 'xo network list'.

```
xo network delete <id> [flags]
```

### Examples

```
  xo network delete 11111111-1111-4111-8111-111111111111
  xo network delete <id> --yes
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

* [xo network](xo_network.md)	 - Manage networks

