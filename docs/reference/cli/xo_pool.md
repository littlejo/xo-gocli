## xo pool

Manage pools

### Synopsis

List and inspect pools, manage their tags, and run the pool-wide
maintenance operations: rolling update, rolling reboot and emergency
shutdown. The maintenance commands are synchronous: they block until the
operation completes (bound the wait with --wait-timeout).

Pools are referenced by their UUID, as printed by 'xo pool list'.

```
xo pool [flags]
```

### Examples

```
  xo pool list
  xo pool get <id>
  xo pool rolling-update <id> --wait-timeout 2h
```

### Options

```
  -h, --help   help for pool
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
* [xo pool emergency-shutdown](xo_pool_emergency-shutdown.md)	 - Shut down every host of the pool immediately
* [xo pool get](xo_pool_get.md)	 - Get a pool
* [xo pool list](xo_pool_list.md)	 - List pools
* [xo pool rolling-reboot](xo_pool_rolling-reboot.md)	 - Reboot the pool's hosts one by one
* [xo pool rolling-update](xo_pool_rolling-update.md)	 - Apply the latest pool update, rolling through the hosts
* [xo pool tag](xo_pool_tag.md)	 - Manage the tags of a pool

