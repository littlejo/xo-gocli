## xo network

Manage networks

### Synopsis

List and inspect networks, create them (on a physical interface, internal
to the pool, or bonded over several interfaces), delete them, and manage
their tags.

Networks are referenced by their UUID, as printed by 'xo network list'.

```
xo network [flags]
```

### Examples

```
  xo network list
  xo network get <id>
  xo network create-internal backend --pool <pool-id>
```

### Options

```
  -h, --help   help for network
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

* [xo](xo.md)	 - Xen Orchestra command line client
* [xo network create](xo_network_create.md)	 - Create a network in a pool
* [xo network create-bonded](xo_network_create-bonded.md)	 - Create a bonded network in a pool
* [xo network create-internal](xo_network_create-internal.md)	 - Create an internal network in a pool
* [xo network delete](xo_network_delete.md)	 - Delete a network
* [xo network get](xo_network_get.md)	 - Get a network
* [xo network list](xo_network_list.md)	 - List networks
* [xo network tag](xo_network_tag.md)	 - Manage the tags of a network

