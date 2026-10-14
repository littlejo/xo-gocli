## xo pbd

Manage physical block devices (PBDs)

### Synopsis

Manage physical block devices (PBDs). A PBD is the connection between a
host and a storage repository: plugging it makes the SR available on that
host, unplugging it disconnects the SR from the host.

PBDs are referenced by their UUID, as printed by 'xo pbd list'.

```
xo pbd [flags]
```

### Examples

```
  xo pbd list
  xo pbd unplug <id> --wait
```

### Options

```
  -h, --help   help for pbd
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
* [xo pbd get](xo_pbd_get.md)	 - Get a physical block device (PBD)
* [xo pbd list](xo_pbd_list.md)	 - List physical block devices (PBDs)
* [xo pbd plug](xo_pbd_plug.md)	 - Plug a PBD (connect the SR to its host)
* [xo pbd unplug](xo_pbd_unplug.md)	 - Unplug a PBD (disconnect the SR from its host)

