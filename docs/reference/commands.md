# Command guide

What `--help` does not say: for each command group, the semantics of the
detail view, which actions are synchronous or asynchronous, what each list
shows, and the caveats. Every flag and subcommand is in the generated
[command reference](cli/xo.md) (also shipped as man pages); each section below
starts with a link to its page there.

For connection profiles (`xo configure`), see
[Configuration](../getting-started/configuration.md#xo-configure); for shell
completion (`xo completion`), see
[Shell completion](../getting-started/shell-completion.md). The output
formats and `--query` are described in
[Output and querying](output-and-query.md), the exit status in
[Exit codes](exit-codes.md).

- [`xo version`](#xo-version)
- [`xo vm`](#xo-vm)
- [`xo host`](#xo-host)
- [`xo sr`](#xo-sr)
- [`xo pool`](#xo-pool)
- [`xo network`](#xo-network)
- [`xo vdi`](#xo-vdi)
- [`xo vbd`](#xo-vbd)
- [`xo pbd`](#xo-pbd)
- [`xo task`](#xo-task)
- [`xo token`](#xo-token)
- [`xo template`](#xo-template)
- [`xo rest`](#xo-rest)

## `xo version`

Reference: [`xo version`](cli/xo_version.md).

Print the CLI version, offline:

```sh
xo version          # or: xo --version
```

The Xen Orchestra server version is not shown: the REST API does not expose
it (only the legacy JSON-RPC API does, which this CLI never uses).

## `xo vm`

Reference: [`xo vm`](cli/xo_vm.md).

Manage virtual machines.

```sh
# Read
xo vm list                          # all VMs
xo vm list --output json            # machine readable
xo vm list --power-state Running    # filter by power state
xo vm list --limit 10               # cap the number of results
xo vm list --query '[].name_label'  # project a single field
xo vm get <id>                      # one VM, as a detail view

# Create
xo vm create web-02 --pool <pool-id> --template <template-id>
xo vm create web-02 --pool <pool-id> --template <template-id> --memory 4G
xo vm create web-02 --pool <pool-id> --template <template-id> --boot

# Update
xo vm update <id> --name web-01
xo vm update <id> --description "primary web server"
xo vm update <id> --tags production,web     # replaces the full tag list

# Tags
xo vm tag add <id> production
xo vm tag remove <id> production

# Disks
xo vm vdis <id>                     # list the VM's VDIs
xo vm vdis <id> --type user         # filter by VDI type
xo vm vdis <id> --query '[].name_label'

# Lifecycle (async actions return a task id; delete is synchronous)
xo vm start <id>                    # power on
xo vm start <id> --host <host-id>   # pin to a host
xo vm start <id> --wait             # block until the start task completes
xo vm stop <id>                     # clean shutdown
xo vm stop <id> --hard              # power off immediately
xo vm reboot <id>                   # clean reboot
xo vm reboot <id> --hard            # force a hard reboot
xo vm pause <id>                    # pause the vCPUs (state: Paused)
xo vm unpause <id>                  # resume a paused VM
xo vm suspend <id>                  # save to disk and release memory (state: Suspended)
xo vm resume <id>                   # restore a suspended VM
xo vm snapshot <id>                 # take a snapshot
xo vm snapshot <id> --name backup   # label the snapshot
xo vm export <id> > vm.xva          # export to XVA (or --file, --format ova)
xo vm import vm.xva --pool <pool>   # import an XVA into a pool
xo vm import vm.xva --pool <pool> --sr <sr-id>   # …with its disks on a given SR
xo vm delete <id>                   # delete the VM (asks to confirm)
xo vm delete <id> --yes             # skip confirmation (automation)
```

`--memory` accepts bytes or human-readable sizes (`2G`, `512M`).

`vm get` shows a single VM as a **detail view** (distinct from `vm list`,
which is the one-line-per-VM table used to pick a VM). It shows identity
(IP, description, tags), location, resources and configuration, and it
resolves the relationships to names: the **container** (the host or pool the
VM runs on) and the **template**. The resolution uses a constant number of
extra lookups (at most one host and one pool, plus the template) and falls
back to the raw id if a reference cannot be resolved, so the view is always
complete. `--output json` / `yaml` / `text` and `--query` still emit the raw
object, unchanged.

```console
$ xo vm get 550e8400-e29b-41d4-a716-446655440001
VM web-01  (Running)
IP:          10.0.0.12
Description: primary web server
Tags:        production, web
Container:   host-02
Template:    Oracle Linux 8
Memory:      4.295GB
CPUs:        2 (max 4)
Disks:       1  (xo vm vdis 550e8400-…)
Networks:    1
Boot:        hvm, order cda
Flags:       hvm auto-poweron
Created:     2026-01-02T10:00:00Z by admin
```

The lifecycle actions (`start`, `stop`, `reboot`, `pause`, `unpause`,
`suspend`, `resume`, `snapshot`) are asynchronous: they return a task id. Add
`--wait` (or set `XOA_WAIT=1`) to block until the task reaches a terminal
state instead; the completed task is then printed (like `xo task wait`) and
the exit status reflects the outcome (non-zero if the task fails or is
interrupted).

The human `POWER STATE` — the `vm list` column and the `vm get` header —
shows the operation in flight while a lifecycle action is running. A VM whose
start has just been requested still reports `Halted` in the raw `power_state`
until the task completes, so the view reads `Starting` instead (likewise
`Shutting down` and `Rebooting`). This is display-only: the `--power-state`
filter is server-side and matches the raw field, and machine output
(`--output json` / `yaml` / `text`) still emits the raw `power_state` with
`current_operations` unchanged.

`vm delete` requires confirmation; pass `--yes` (or set `XOA_YES`) to run
non-interactively. Without either, a non-terminal stdin is rejected rather
than hanging, so automation never blocks. The power actions (`start`, `stop`,
`reboot`, `pause`, `unpause`, `suspend`, `resume`) never prompt: each is
reversible by its counterpart (`stop` by `start`, `suspend` by `resume`, …),
so a confirmation would only interrupt the normal start/stop/reboot loop —
matching the AWS CLI, where none of these ask.

`vm export` streams the archive to stdout by default (use `--file` for a
file, `--format ova` for OVA, `--compress=false` to disable XVA
compression); `vm import` reads the XVA from a file or stdin (`-`) and
requires `--pool`; `--sr` puts the VM's disks on an SR of that pool instead
of the pool's default SR. Both are performed through the SDK's own REST client
because the typed SDK service does not expose them yet (see
[SDK v2 notes](../internals/sdk-v2.md#known-sdk-gaps-the-cli-works-around)).

## `xo host`

Reference: [`xo host`](cli/xo_host.md).

Manage hosts.

```sh
xo host list
xo host list --query '[].name_label'
xo host list --query '[?power_state==`Running`].name_label'
xo host get <id>                 # one host, as a detail view

# Tags
xo host tag add <id> production
xo host tag remove <id> production
```

`host get` shows a single host as a **detail view** (distinct from `host
list`). It shows the address and hostname, the **pool** the host belongs to
(resolved by name), the **memory** usage, the CPU model, the platform and
license, and the **VMs** resident on the host (resolved in one batch, never
one lookup per VM). If the pool cannot be resolved, the raw id is shown
instead. `--output json` / `yaml` / `text` and `--query` still emit the raw
object, unchanged.

```console
$ xo host get aaaaaaaa-bbbb-cccc-dddd-000000000001
Host host-01  (Running)
Address:     10.0.0.11
Pool:        prod-pool
Memory:      62.50GB / 128GB (48%)
CPUs:        32 cores, 4 sockets
CPU:         Intel(R) Xeon(R) CPU E5-2680 v4 @ 2400 MHz
Platform:    XCP-ng 8.2.0
VMs:         14  (web-01, db-01, …  (+12 more))
```

## `xo sr`

Reference: [`xo sr`](cli/xo_sr.md).

Manage storage repositories.

```sh
xo sr list
xo sr list --type lvm               # filter by SR type (see note below)
xo sr list --query '[?SR_type==`nfs`].name_label'
xo sr get <id>
xo sr scan <id>                     # rescan the SR for disk changes (async)
xo sr scan <id> --wait              # …and wait for the scan to finish
xo sr reclaim-space <id>            # reclaim unused (thin) space (async)
xo sr reclaim-space <id> --wait

# Tags
xo sr tag add <id> production
xo sr tag remove <id> production
```

`--type` is passed to the XO live-filter engine, which is a case-insensitive
substring match: `--type lvm` also matches `lvmoiscsi`. For an exact type,
project with `--query` instead.

`sr get` shows a single SR as a **detail view** (distinct from `sr list`):
the **container** (pool or host, resolved by name), the type, content type,
sharing and allocation, the size and usage, the counts of VDIs and PBDs, and
the hosts the SR is **connected to** (resolved in one batch, never one lookup
per host). A reference that cannot be resolved shows the raw id, and a line
that cannot be computed is omitted. `--output json` / `yaml` / `text` and
`--query` still emit the raw object, unchanged.

## `xo pool`

Reference: [`xo pool`](cli/xo_pool.md).

Manage pools.

```sh
xo pool list
xo pool list --query '[?HA_enabled].name_label'
xo pool get <id>                 # one pool, as a detail view

# Maintenance (all synchronous: the command waits for the backing task)
xo pool rolling-update <id>                  # apply the pool update, host by host
xo pool rolling-reboot <id>                  # reboot the hosts one by one (confirm + --yes)
xo pool emergency-shutdown <id>              # shut down every host at once (confirm + --yes)

# Tags
xo pool tag add <id> production
xo pool tag remove <id> production
```

`rolling-update` applies the latest pool update, rolling it across the hosts
one by one (each host is rebooted in turn, with its VMs moved away first);
the pool stays available the whole time and the command runs without
confirmation. `rolling-reboot` reboots the hosts in the same rolling fashion
but is destructive and asks for confirmation. `emergency-shutdown` powers off
every host at once **without evacuating the VMs first**: the pool is down
afterwards, so it is a last resort. Like the other destructive commands,
`--yes` (or `$XOA_YES=1`) skips the confirmation. All three block until the
backing task completes (or Ctrl+C); there is no default wait deadline, and
`--wait-timeout` sets one — see [Timeouts and
waiting](../getting-started/configuration.md#timeouts-and-waiting).

`pool get` shows a single pool as a **detail view** (distinct from `pool
list`). It shows the **master** host (resolved by name), the pool's **hosts**
(resolved in one batch, never one lookup per host), the storage
repositories — **default**, suspend and crash-dump (resolved by name) — and
the platform, CPU topology and features. If a reference cannot be resolved,
the raw id is shown instead. `--output json` / `yaml` / `text` and `--query`
still emit the raw object, unchanged.

```console
$ xo pool get 550e8400-e29b-41d4-a716-446655440001
Pool prod-pool  (HA)
Master:      host-master
Hosts:       host-01, host-02
Default SR:  Local Storage
Platform:    8.2
CPUs:        16 cores, 2 sockets
Features:    auto-poweron zstd
```

## `xo network`

Reference: [`xo network`](cli/xo_network.md).

Manage networks.

```sh
xo network list
xo network list --query '[].name_label'
xo network get <id>                 # one network, as a detail view

# Create (the creation is asynchronous server-side; the command waits for the
# backing task and prints the created network)
xo network create <name> --pool <pool-id> --pif <pif-id>
xo network create <name> --pool <pool-id> --pif <pif-id> --vlan 100 --mtu 9000
xo network create <name> --pool <pool-id> --pif <pif-id> --description "storage" --nbd
xo network create-internal <name> --pool <pool-id>
xo network create-bonded <name> --pool <pool-id> --pifs <pif-1>,<pif-2> --bond-mode active-backup

# Delete (confirm + --yes)
xo network delete <id>
xo network delete <id> --yes

# Tags
xo network tag add <id> production
xo network tag remove <id> production
```

`create` attaches the network to a PIF (physical interface) of one of the
pool's hosts; `--vlan` selects the VLAN tag (0 for untagged). `create-internal`
creates a network with no physical attachment, carrying virtual traffic
between VMs. `create-bonded` links several PIFs into one logical network;
`--bond-mode` is `active-backup`, `balance-slb` or `lacp`. PIFs are listed
with `xo rest get pifs` (there is no typed PIF command yet).

The three create commands share three optional flags: `--mtu` (0, the
default, means the XenServer default of 1500), `--description`, and `--nbd`,
which allows NBD (Network Block Device) access to the network, as used by
backups that read disks over NBD.

`network get` shows a single network as a **detail view** (distinct from
`network list`). It shows the bridge, MTU and type, the **pool** the network
belongs to (resolved by name), and the counts of VIFs and PIFs. The PIF
hosts are not listed because PIFs have no typed SDK service yet; use
`xo rest get pifs --param filter=<network-id>` to see them. `--output json` /
`yaml` / `text` and `--query` still emit the raw object, unchanged.

```console
$ xo network get 11111111-1111-4111-8111-111111111111
Network Management  (external)
Pool:      prod-pool
Bridge:    xenbr0
MTU:       1500
Default:   locked
VIFs:      12
PIFs:      2
```

## `xo vdi`

Reference: [`xo vdi`](cli/xo_vdi.md).

Manage virtual disks (VDIs). A VDI is a disk that lives on a storage
repository (SR); it is not attached to a VM until you create a VBD for it.

```sh
xo vdi list
xo vdi list --type user
xo vdi list --query '[].name_label'
xo vdi get <id>                 # one VDI, as a detail view

# Create / delete (delete is confirm + --yes)
xo vdi create data-01 --sr <sr-id> --size 10G
xo vdi create data-01 --sr <sr-id> --size 10G --description "data disk" --shared --tags common
xo vdi delete <id>
xo vdi delete <id> --yes

# Migrate to another SR (async: prints a task id; the VDI gets a NEW id)
xo vdi migrate <id> --sr <sr-id>
xo vdi migrate <id> --sr <sr-id> --output json

# Tags
xo vdi tag add <id> production
xo vdi tag remove <id> production

# Export / import (raw or vhd; export streams to stdout unless --file is given)
xo vdi export <id> > disk.raw
xo vdi export <id> --file disk.raw
xo vdi export <id> --format vhd --file disk.vhd
xo vdi import <id> disk.raw          # confirm + --yes (overwrites the VDI)
cat disk.raw | xo vdi import <id> -
```

`--size` accepts bytes or a human readable size (e.g. `2G`, `512M`). Creating
a VDI only allocates it on the SR; attach it to a VM with `xo vbd create`
(see the [use cases](../usecases.md#add-a-disk-to-a-vm)).

`xo vdi migrate` is asynchronous: it prints the task id and returns. Track it
with `xo task get <task-id>` or `xo task wait <task-id>`. When the migration
completes the VDI has a **new id**, so re-look it up by name (`xo vdi list`)
or through its VM (`xo vm vdis <vm-id>`).

`vdi get` shows a single VDI as a **detail view** (distinct from `vdi list`):
the **SR** (resolved by name), the type, size and usage, the snapshot count,
and the VMs the VDI is **attached to** (resolved in one batch; beyond 10
names the sheet shows a `(+N more)` count). `--output json` / `yaml` /
`text` and `--query` still emit the raw object, unchanged.

`xo vdi import` overwrites the content of an existing VDI, so it asks for
confirmation unless `--yes` (or `$XOA_YES=1`) is given. The target VDI already
exists (create it first with `xo vdi create` if needed).

## `xo vbd`

Reference: [`xo vbd`](cli/xo_vbd.md).

Manage virtual block devices (VBDs). A VBD is the attachment point between a
VM and a VDI — it is what "plugs a disk into a VM".

```sh
xo vbd list
xo vbd list --vm <vm-id>        # only the VBDs of one VM
xo vbd list --query '[].VDI'
xo vbd get <id>                 # one VBD, as a detail view

# Attach / detach
xo vbd create --vm <vm-id> --vdi <vdi-id>
xo vbd create --vm <vm-id> --vdi <vdi-id> --mode RO --bootable
xo vbd delete <id>              # detach (keeps the VDI); confirm + --yes
xo vbd delete <id> --yes

# Hot-plug / hot-unplug (running VM; async, --wait to block)
xo vbd connect <id>
xo vbd connect <id> --wait
xo vbd disconnect <id>
xo vbd disconnect <id> --wait
```

`create` links a VDI to a VM; it does not hot-plug. If the VM is running, run
`connect` afterwards so the guest sees the disk without a reboot. `delete`
removes only the attachment — the VDI (and its data) is kept; use
`xo vdi delete` to remove the disk itself. `connect` / `disconnect` are
asynchronous (they print a task id); `--wait` blocks until the task completes.

Like `vm get`, `vbd get` shows a single VBD as a **detail view** (distinct
from the `vbd list` table). It resolves the two relationships by name — the
VM the VBD belongs to, and the VDI it points at (with its size) — at a constant
cost (one VM lookup and one VDI lookup). `--output json` / `yaml` / `text` and
`--query` still emit the raw object, unchanged.

```console
$ xo vbd get 33333333-3333-4333-8333-333333333333
VBD xvda  (RW, attached=yes, bootable)
Device:    xvda
Position:  xvdb
VM:        web-01
VDI:       sys-disk
Size:      42.95GB
```

## `xo pbd`

Reference: [`xo pbd`](cli/xo_pbd.md).

Manage physical block devices (PBDs). A PBD is the connection between a host
and a storage repository (SR) — it is what "plugs" an SR into a host.

```sh
xo pbd list
xo pbd list --query '[].attached'
xo pbd get <id>                 # one PBD, as a detail view

# Connect / disconnect the SR to its host (async: prints a task id; --wait to block)
xo pbd plug <id>
xo pbd plug <id> --wait
xo pbd unplug <id>
xo pbd unplug <id> --wait
```

`plug` / `unplug` are asynchronous: they print the task id and return. Track
them with `xo task get <task-id>` or `xo task wait <task-id>`, or add `--wait`
to block until the task completes. A PBD's `attached` column reflects whether
the SR is currently connected.

Like `vm get`, `pbd get` shows a single PBD as a **detail view** (distinct
from the `pbd list` table). It resolves the three relationships by name — the
host, the SR and the pool — at a constant cost (one lookup each), and shows the
**full** `device_config`, not just the block device: for an NFS SR you get the
`server` and `serverpath`, for a local disk the `device`. `--output json` /
`yaml` / `text` and `--query` still emit the raw object, unchanged.

```console
$ xo pbd get 66666666-6666-4666-8666-666666666666
PBD /dev/sda  (attached=yes)
Host:        host-01
SR:          Local Storage
Pool:        prod-pool
Config:      device=/dev/sda
```

## `xo task`

Reference: [`xo task`](cli/xo_task.md).

Manage asynchronous tasks.

```sh
xo task list                        # all asynchronous tasks
xo task list --status failure       # filter by status (pending, success, failure, interrupted)
xo task list --query '[].id'
xo task get <id>                    # one task, as a detail view
xo task wait <id>                   # block until the task completes
xo task wait <id> --wait-timeout 5m # …but give up after 5 minutes
xo task abort <id>                  # ask Xen Orchestra to interrupt a running task
xo task abort <id> --yes            # …without the confirmation prompt
```

`task get` shows a single task as a **detail view** (distinct from `task
list`): the status, the operation (type, name, method), the user that ran it
and the object it targeted, when it started and ended, its duration and any
subtasks. For a **failed** task the error (code and message) is shown; for a
successful one with a result, that result is shown. `--output json` / `yaml` /
`text` and `--query` still emit the raw object, unchanged.

```console
$ xo task get f6e5d4c3b2a1
Task f6e5d4c3b2a1  (failure)
Type:      VM
Name:      clean_shutdown
Target:    550e8400-e29b-41d4-a716-446655440002
Started:   2026-09-28T10:05:00Z
Ended:     2026-09-28T10:05:02Z
Duration:  2s
Error:     VM_NOT_FOUND — VM not found
```

Asynchronous operations (`vm start`, `vm stop`, …) return a task id; follow
it with `xo task get <id>` or `xo task wait <id>`. `vm create` and the
`network create` commands are different: they wait for the creation task
inside the SDK and only return once the resource exists, printing the full
object (see [Mutation and action output](output-and-query.md#mutation-and-action-output)).

`task abort` requests the interruption of a task that is still **pending**;
the task then reaches the `interrupted` status. It is a destructive operation
(aborting a VM start mid-way, for example, does not always fully unwind the
operation) and asks for confirmation unless `--yes` (or `$XOA_YES=1`) is
given. Aborting a task that already reached a terminal state (`success`,
`failure` or `interrupted`) is rejected with a clear error.

`task wait` polls the task every 2 seconds until it reaches a terminal state
(`success`, `failure` or `interrupted`) and prints it like `task get`. It is
meant to be scripted: the exit status reflects the outcome — `0` on success,
non-zero if the task fails, is interrupted, the `--wait-timeout` deadline is hit,
or the task does not exist — while the completed task is still written to
stdout in the chosen format.

`--wait-timeout` bounds the whole wait; the global `--timeout` (or
`$XOA_TIMEOUT`) bounds each poll request.

## `xo token`

Reference: [`xo token`](cli/xo_token.md).

Manage authentication tokens.

These are the tokens of the current user (the same value `xo configure`
stores). The token **id is the secret**, so it is masked in the output by
default — pass `--no-secret` to reveal it (use with care).

```sh
xo token list                         # your tokens (id masked)
xo token list --no-secret --query '[].id'
xo token get <id>                     # one token (resolved against the list)
xo token create                       # create a token, printed in full once
xo token create --description "ci" --expires-in "30 days"
xo token create --client-id my-cli    # reuse the token for a given client
```

`create` prints the token in full **once** (save it, e.g. into `xo
configure`); `list`/`get` only show a masked id. Deletion is not exposed by
the REST API and is therefore not implemented here.

## `xo template`

Reference: [`xo template`](cli/xo_template.md).

Manage VM templates.

In Xen Orchestra, templates are first-class objects (REST resource
`vm-templates`), not part of the `vms` collection — so `xo vm list` does not
show them. `xo template list` reads that dedicated resource.

```sh
xo template list
xo template list --output json
xo template list --query '[].name_label'
xo template get <id>            # one template, as a detail view
```

`template get` shows a single template as a **detail view** (distinct from
`template list`): the memory and CPUs, the power state, and the **pool** the
template belongs to (resolved by name). If the pool cannot be resolved, the
raw id is shown instead. `--output json` / `yaml` / `text` and `--query`
still emit the raw object, unchanged.

```console
$ xo template get d31e47fd-a70e-d849-883e-c17193472710-6959dfe8-534c-4c58-8a8c-3c3792293543
Template Oracle Linux 8  (default)
Pool:        prod-pool
Memory:      4.295GB
CPUs:        2
Power state: Halted
```

## `xo rest`

Reference: [`xo rest`](cli/xo_rest.md).

Call a raw Xen Orchestra REST endpoint.

Low-level escape hatch for Xen Orchestra REST endpoints that have no typed
command yet. Use it for what the typed commands don't cover — users and
groups, SR actions, … — not to duplicate `xo vm list` when `xo vm list`
exists. The request goes through the SDK v2 HTTP client (same authentication,
base URL and TLS handling), so it is not a second REST client. Prefer the
typed commands when they cover what you need.

The path is relative to the REST API root (`/rest/v0`), and the full OpenAPI
spec at `<endpoint>/rest/v0/docs` lists every endpoint and its fields. See
also the [official REST API documentation](https://docs.xen-orchestra.com/automation/restapi).

```sh
# Resources with no typed command yet
xo rest get users --output json                # list users
xo rest get groups                             # list groups

# Actions without a typed command
xo rest post srs/<id>/actions/scan             # rescan an SR
xo rest post srs/<id>/actions/reclaim_space    # reclaim free space on an SR

# Raw access to a typed resource (when the typed command lacks a field/option)
xo rest get vdis --output json                 # raw VDI fields (typed: 'xo vdi list')
xo rest get pifs                               # PIFs have no typed command yet

# Request building
xo rest get vdis --param limit=10              # add query parameters
xo rest post pools/<id>/actions/create_network --data '{...}'   # JSON body
xo rest post vdis --data - < vdi.json          # read the body from stdin
xo rest delete vdis/<id> --yes                 # destructive: asks unless --yes
xo rest get vdis --output json --query '[].name_label'
xo rest get vdis -i                             # status line + headers on stderr
```

Flags: `--data` (JSON body, `-` = stdin), `--param KEY=VALUE` (repeatable),
`--header KEY: VALUE` (repeatable), `--query/-q`, `--yes`, `--include/-i`.

More resources and sub-commands (`get`, `start`, `stop`, …) are added on top of
the SDK as it evolves. See `xo <resource> --help` for the current surface.
