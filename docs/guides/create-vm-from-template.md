# Create a VM from a template

Provision a new virtual machine from one of the templates of a pool, start
it, read its IP address, and delete it when you no longer need it.

```text
   1. find the pool                   xo pool list
   2. find a template of that pool    xo template list
   3. create the VM                   xo vm create
   4. check it                        xo vm get
   5. start it                        xo vm start   (or --boot at step 3)
   6. read its IP address             xo vm get --query mainIpAddress
```

## Prerequisites

- A configured profile that can reach Xen Orchestra (see
  [Configuration](../getting-started/configuration.md)).
- A template that already defines the disks and network interfaces you want.
  `xo vm create` takes the CPU count, disks and network interfaces from the
  template; only the name, description and memory can be set on the command
  line.
- Guest tools installed in the template's OS if you want Xen Orchestra to
  report the VM's IP address (step 6).

## Step 1: find the pool

```sh
xo pool list
```

The table has the columns `ID NAME VERSION CORES SOCKETS MASTER HA`. Copy the
`ID` of the pool that will host the VM: `xo vm create` takes the pool by UUID,
not by name. To get it by name in a script:

```sh
xo pool list --query "[?name_label=='pool-01'].id | [0]" --output text
```

## Step 2: find a template

```sh
xo template list
```

The columns are `ID NAME DEFAULT MEMORY CPUS POOL`. Templates are not VMs, so
`xo vm list` does not show them. The `ID` column is a composite
`<poolId>-<templateUuid>` id; `--template` accepts it as is, and also accepts
the bare template UUID (the `uuid` field of `--output json`). The template
must belong to the pool chosen in step 1 (check the `POOL` column).

Narrow the list down with `--query`:

```sh
xo template list --query "[?contains(name_label, 'Debian')].{id: id, name: name_label}"
```

## Step 3: create the VM

```sh
xo vm create web-02 --pool <pool-id> --template <template-id> --memory 4G --description "web server"
```

`--memory` takes bytes or a human readable size (`2G`, `512M`); without it,
the template default is used. The creation is asynchronous on the server; the
command waits for the backing task, then prints the new VM's name, `id`,
state, memory and CPU count. A halted VM comes with a `Start it with: xo vm
start <id>` hint.

> The wait of `xo vm create` has no deadline: it ends when the task completes
> or on Ctrl+C. See [Troubleshooting](../troubleshooting.md) if it seems stuck.

Add `--boot` to start the VM right after its creation and skip step 5.

## Step 4: check it

```sh
xo vm get <vm-id>
```

The default view is a detail sheet (identity, the host or pool it runs on,
resources and configuration). For a single field, use `--query`:

```sh
xo vm get <vm-id> --query power_state --output text
```

## Step 5: start it

```sh
xo vm start <vm-id> --wait
```

Without `--wait`, the command returns as soon as the start is requested and
prints the task id; with it, it waits for the task and prints the finished
task. Use `--host <host-id>` to pin the VM to a specific host.

## Step 6: read its IP address

```sh
xo vm get <vm-id> --query mainIpAddress --output text
```

The `mainIpAddress` field is filled in by the guest tools once the OS has
booted and obtained an address. Until then the field is absent and the
command prints nothing. `xo vm list` shows the same value in its `IP` column.

## Undo: delete the VM

```sh
xo vm stop <vm-id> --wait
xo vm delete <vm-id> --yes
```

A running VM must be stopped (or suspended) before it can be deleted. The
deletion is irreversible and removes the VM's disks; without `--yes`, `xo`
asks for confirmation (and refuses when stdin is not a terminal).

## Scripted version

```sh
set -euo pipefail
POOL=$(xo pool list --query "[?name_label=='pool-01'].id | [0]" --output text)
TEMPLATE=$(xo template list --query "[?name_label=='Debian Bookworm 12'].id | [0]" --output text)

# Create and boot; keep only the new VM id
VM_ID=$(xo vm create web-02 --pool "$POOL" --template "$TEMPLATE" --memory 4G --boot --output json | jq -r '.id')

# Wait up to 5 minutes for the guest tools to report an address
IP=""
for _ in $(seq 1 30); do
  IP=$(xo vm get "$VM_ID" --query mainIpAddress --output text)
  [ -n "$IP" ] && break
  sleep 10
done
echo "VM $VM_ID is up at ${IP:-<no address reported>}"
```

`xo vm create --output json` prints the created VM object, so `.id`,
`.name_label` and `.power_state` are available to `jq`.

## Troubleshooting

| Symptom | Likely cause / fix |
| ------- | ------------------ |
| `--pool is required (see 'xo pool list')` / `--template is required (see 'xo template list')` | Both flags are mandatory. |
| `invalid --pool id "..." (expected a UUID)` | `--pool` takes the pool UUID, not its name. See step 1. |
| `invalid --template id "..." (expected a UUID, or the composite poolId-templateUuid id printed by 'xo template list')` | Copy the `ID` column of `xo template list` as is, or use the bare template UUID. |
| `cannot create VM "...": ...` | The server refused the creation; the most common reason is a template that does not belong to `--pool`. Add `--debug` for the API detail. |
| `xo vm create` does not return | The creation task is still running (or stuck). Check `xo task list --status pending`; Ctrl+C stops the wait. |
| No IP address | The guest tools are not installed or the VM has not finished booting. |
| `cannot delete VM ...` | The VM is still running: `xo vm stop <vm-id> --wait` first. |
