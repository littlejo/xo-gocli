## xo

Xen Orchestra command line client

### Synopsis

xo is a command line client for Xen Orchestra.

It talks to the Xen Orchestra REST API through the official Go SDK (v2).
Configure a profile first with 'xo configure'.

### Options

```
  -d, --debug              show SDK/API error details on failure (or $XOA_DEBUG)
  -h, --help               help for xo
      --json               output as JSON (shorthand for --output json)
  -o, --output string      output format: table, json, yaml, text (or $XOA_DEFAULT_OUTPUT, or the profile's "output" value) (default "table")
  -p, --profile string     name of the configuration profile to use (or $XOA_PROFILE)
      --timeout duration   HTTP client timeout, e.g. 60s or 2m (default 30s, or $XOA_TIMEOUT)
      --version            print the xo CLI version
```

### SEE ALSO

* [xo configure](xo_configure.md)	 - Manage configuration profiles
* [xo host](xo_host.md)	 - Manage hosts
* [xo network](xo_network.md)	 - Manage networks
* [xo pbd](xo_pbd.md)	 - Manage physical block devices (PBDs)
* [xo pool](xo_pool.md)	 - Manage pools
* [xo rest](xo_rest.md)	 - Call a raw Xen Orchestra REST endpoint
* [xo sr](xo_sr.md)	 - Manage storage repositories
* [xo task](xo_task.md)	 - Manage asynchronous tasks
* [xo template](xo_template.md)	 - Manage VM templates
* [xo token](xo_token.md)	 - Manage authentication tokens
* [xo vbd](xo_vbd.md)	 - Manage virtual block devices (VBDs)
* [xo vdi](xo_vdi.md)	 - Manage virtual disks (VDIs)
* [xo version](xo_version.md)	 - Print the xo CLI version
* [xo vm](xo_vm.md)	 - Manage virtual machines

