## xo host

Manage hosts

### Synopsis

List and inspect the hosts (XCP-ng or XenServer servers) that Xen
Orchestra manages, and manage their tags.

Hosts are referenced by their UUID, as printed by 'xo host list'.

```
xo host [flags]
```

### Examples

```
  xo host list
  xo host get <id>
```

### Options

```
  -h, --help   help for host
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
* [xo host get](xo_host_get.md)	 - Get a host
* [xo host list](xo_host_list.md)	 - List hosts
* [xo host tag](xo_host_tag.md)	 - Manage the tags of a host

