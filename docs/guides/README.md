# Guides

Task-oriented walkthroughs that chain several `xo` commands to get a real job
done. Each guide lists its prerequisites, the steps with their commands, how
to undo the result, a scripted version and a short troubleshooting table.
For a single command and its flags, see the
[command reference](../reference/commands.md); for error messages, see
[Troubleshooting](../troubleshooting.md).

| Guide | What you will do |
| ----- | ---------------- |
| [Add a disk to a VM](../usecases.md) | Create a VDI, attach it to a VM, hot-plug it and prepare it in the guest. |
| [Create a VM from a template](create-vm-from-template.md) | Pick a pool and a template, create and start a VM, read its IP address, delete it. |
| [Snapshot and export a VM](snapshot-and-export-vm.md) | Take a snapshot, export the VM to an XVA/OVA file and import it into another pool. |
| [Work with several pools and profiles](multiple-pools-profiles.md) | Store one profile per Xen Orchestra instance, switch between them and loop over all of them. |
| [Automate `xo` in CI](ci-automation.md) | Run `xo` from a pipeline with environment-only configuration, JSON output, exit codes and task gates. |
| [Pool rolling update and reboot](pool-rolling-update.md) | Apply pool updates host by host, follow the task, and know what to do if it is interrupted. |
| [Bulk actions with `--query`](bulk-actions-with-query.md) | Select many objects with JMESPath and stop, start or tag them safely. |
