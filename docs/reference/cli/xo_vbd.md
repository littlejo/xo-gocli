## xo vbd

Manage virtual block devices (VBDs)

### Synopsis

Manage virtual block devices (VBDs). A VBD is the attachment between a VM
and a VDI: creating one plugs a disk into a VM, deleting it unplugs the
disk without deleting the VDI.

VBDs are referenced by their UUID, as printed by 'xo vbd list'.

```
xo vbd [flags]
```

### Examples

```
  xo vbd list
  xo vbd create --vm <vm-id> --vdi <vdi-id>
  xo vbd connect <id> --wait
```

### Options

```
  -h, --help   help for vbd
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
* [xo vbd connect](xo_vbd_connect.md)	 - Hot-plug a VBD (attach the disk to a running VM)
* [xo vbd create](xo_vbd_create.md)	 - Attach a VDI to a VM
* [xo vbd delete](xo_vbd_delete.md)	 - Detach a VDI from a VM (delete the VBD)
* [xo vbd disconnect](xo_vbd_disconnect.md)	 - Hot-unplug a VBD (detach the disk from a running VM)
* [xo vbd get](xo_vbd_get.md)	 - Get a virtual block device (VBD)
* [xo vbd list](xo_vbd_list.md)	 - List virtual block devices (VBDs)

