## xo sr

Manage storage repositories

### Synopsis

List and inspect storage repositories (SRs), the storage that holds the
virtual disks (VDIs); rescan them, reclaim their free space, and manage
their tags.

SRs are referenced by their UUID, as printed by 'xo sr list'.

```
xo sr [flags]
```

### Examples

```
  xo sr list
  xo sr get <id>
  xo sr scan <id> --wait
```

### Options

```
  -h, --help   help for sr
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
* [xo sr get](xo_sr_get.md)	 - Get a storage repository
* [xo sr list](xo_sr_list.md)	 - List storage repositories
* [xo sr reclaim-space](xo_sr_reclaim-space.md)	 - Reclaim unused space from a storage repository
* [xo sr scan](xo_sr_scan.md)	 - Scan a storage repository
* [xo sr tag](xo_sr_tag.md)	 - Manage the tags of a storage repository

