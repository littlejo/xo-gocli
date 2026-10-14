## xo vm

Manage virtual machines

### Synopsis

Manage virtual machines: list and inspect them, create, update and delete
them, drive their power state, snapshot, export and import them, and manage
their tags.

Lifecycle actions (start, stop, reboot, pause, suspend, ...) are
asynchronous: they print a task id and return. Add --wait (or set
$XOA_WAIT) to block until the task completes.

VMs are referenced by their UUID, as printed by 'xo vm list'.

```
xo vm [flags]
```

### Examples

```
  xo vm list
  xo vm list --query "[?power_state=='Running'].name_label"
  xo vm get <id>
  xo vm start <id> --wait
```

### Options

```
  -h, --help   help for vm
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
* [xo vm create](xo_vm_create.md)	 - Create a virtual machine in a pool
* [xo vm delete](xo_vm_delete.md)	 - Delete a virtual machine
* [xo vm export](xo_vm_export.md)	 - Export a virtual machine to an XVA or OVA archive
* [xo vm get](xo_vm_get.md)	 - Get a virtual machine
* [xo vm import](xo_vm_import.md)	 - Import a virtual machine from an XVA archive
* [xo vm list](xo_vm_list.md)	 - List virtual machines
* [xo vm pause](xo_vm_pause.md)	 - Pause a virtual machine
* [xo vm reboot](xo_vm_reboot.md)	 - Reboot a virtual machine
* [xo vm resume](xo_vm_resume.md)	 - Resume a suspended virtual machine
* [xo vm snapshot](xo_vm_snapshot.md)	 - Take a snapshot of a virtual machine
* [xo vm start](xo_vm_start.md)	 - Start a virtual machine
* [xo vm stop](xo_vm_stop.md)	 - Stop a virtual machine
* [xo vm suspend](xo_vm_suspend.md)	 - Suspend a virtual machine
* [xo vm tag](xo_vm_tag.md)	 - Manage the tags of a virtual machine
* [xo vm unpause](xo_vm_unpause.md)	 - Unpause a virtual machine
* [xo vm update](xo_vm_update.md)	 - Update the name, description or tags of a virtual machine
* [xo vm vdis](xo_vm_vdis.md)	 - List the virtual disks (VDIs) of a virtual machine

