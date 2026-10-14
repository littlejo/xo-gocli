## xo vbd create

Attach a VDI to a VM

### Synopsis

Attach a virtual disk (VDI) to a virtual machine (VM) by creating a VBD.

The VM and the VDI are referenced by their UUID, as returned by
'xo vm list' and 'xo vdi list' (or 'xo vm vdis <id>') respectively. The VDI
must live on an SR that is shared by the pool the VM belongs to, or on a
local SR of the host the VM runs on.

Creating the VBD does not hot-plug the disk: if the VM is running, hot-plug
it afterwards with 'xo vbd connect <id>' (or reboot the VM). Detach it with
'xo vbd delete <id>' (this only removes the attachment; the VDI itself is
kept).

```
xo vbd create [flags]
```

### Examples

```
  xo vbd create --vm <vm-id> --vdi <vdi-id>
  xo vbd create --vm <vm-id> --vdi <vdi-id> --mode RO
  xo vbd create --vm <vm-id> --vdi <vdi-id> --bootable
```

### Options

```
      --bootable      make the VBD a boot device
  -h, --help          help for create
      --mode string   access mode: RO or RW (default RW)
      --vdi string    VDI UUID to attach (required, see 'xo vdi list')
      --vm string     VM UUID to attach the VDI to (required, see 'xo vm list')
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

* [xo vbd](xo_vbd.md)	 - Manage virtual block devices (VBDs)

