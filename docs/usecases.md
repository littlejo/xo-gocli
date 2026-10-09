# Use cases

Task-oriented recipes that put several `xo` commands together to achieve
something real. Each use case shows the commands in order, the expected output,
and how to script it. For the full command reference see
[usage.md](usage.md); for the architecture see [development.md](development.md).

A use case is *complete* when every step is a typed `xo` command (no `xo rest`
needed) and the final state is verified.

## Table of contents

- [Add a disk to a VM](#add-a-disk-to-a-vm)
- [Create a VM until it is SSH-reachable](#create-a-vm-until-it-is-ssh-reachable)

---

## Add a disk to a VM

Attach a new virtual disk (VDI) to a running or halted VM — the equivalent of
adding a second hard drive. This is the most common storage task and touches
two resources: the **VDI** (the disk, which lives on a storage repository) and
the **VBD** (the attachment that plugs the disk into the VM).

```text
   1. find a target SR                xo sr list
   2. create the VDI  on an SR        xo vdi create
   3. attach it to the VM  (VBD)      xo vbd create
   4. hot-plug it (VM is running)     xo vbd connect
   5. verify on the host              xo vm vdis / xo vbd list
   6. prepare it in the guest         lsblk, mkfs, mount, fstab
```

Steps 1–5 are done with `xo` (the Xen Orchestra side, i.e. the virtual
hardware); step 6 is done **inside the guest** — the disk is a plain block
device there, and the OS must format and mount it like any physical disk.

### Prerequisites

- A configured profile (`xo configure`) that can reach the pool.
- A **storage repository (SR)** the VM can see — usually the pool's shared SR.
  List them with `xo sr list`. A VDI can only live on an SR that is available
  to the hosts of the VM's pool (a local SR works only if the VM runs on that
  host).
- The VM's UUID (`xo vm list`) and, if the disk is bootable or read-only, a
  decision about `--bootable` / `--mode`.

### Step 1 — find a target SR and size

```sh
xo sr list
```

```
ID                                    NAME           TYPE  SIZE   USAGE  CONTAINER
------------------------------------  -------------  ----  -----  -----  ---------
aaaaaaaa-bbbb-cccc-dddd-000000000001  Local storage  lvm   200GB  95GB   pool-01
bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb  NFS data       nfs   2TB    1.2TB  pool-01
```

The reference columns of a `list` table (here `CONTAINER`) show the resolved
**name** of the referenced object (the pool the SR belongs to, or the host when
the SR is host-local) instead of the raw UUID. Machine output
(`--output json`) keeps the raw references.

Pick the shared SR (e.g. `bbbbbbbb-…`) and a virtual size (`10G`, `50G`, or a
raw byte count).

### Step 2 — create the VDI

```sh
xo vdi create data-01 --sr bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb --size 10G
```

```
VDI created (id 44444444-4444-4444-8444-444444444444)

Attach it to a VM with: xo vbd create --vm <vm-id> --vdi 44444444-4444-4444-8444-444444444444
```

The created VDI is **not** attached to anything yet. Useful flags:
`--description`, `--tags a,b`, and `--shared` (let several VMs attach the same
VDI at once, read/write — only sensible on a shared SR and a VM that supports
it).

> The VDI id is what you need for the next step. Capture it in a script
> (`--output json` makes this robust, see below).

### Step 3 — attach it to the VM (create the VBD)

```sh
xo vbd create --vm 550e8400-e29b-41d4-a716-446655440001 --vdi 44444444-4444-4444-8444-444444444444
```

```
VBD created (id 55555555-5555-4555-8555-555555555555): VDI 44444444-4444-4444-8444-444444444444 attached to VM 550e8400-e29b-41d4-a716-446655440001

If the VM is running, hot-plug the disk with: xo vbd connect 55555555-5555-4555-8555-555555555555
```

Creating the VBD does **not** make the disk appear inside a running guest —
that is the hot-plug in the next step. Optional flags: `--mode RO` (read-only
attachment) and `--bootable` (make this a boot device).

### Step 4 — hot-plug (only if the VM is running)

```sh
xo vbd connect 55555555-5555-4555-8555-555555555555
```

```
Requested connect of VBD 55555555-5555-4555-8555-555555555555 (VM 550e8400-e29b-41d4-a716-446655440001, VDI 44444444-4444-4444-8444-444444444444) (task task-123)
```

- **VM running** → run `xo vbd connect` to attach the disk without a reboot.
  The guest OS then sees a new block device (e.g. `/dev/xvdb` on a typical
  Linux guest).
- **VM halted** → skip this step; the disk is attached automatically at boot.

To remove the disk later without deleting it, hot-unplug it first with
`xo vbd disconnect <vbd-id>` (running VM), then detach with
`xo vbd delete <vbd-id> --yes`, and finally remove the disk with
`xo vdi delete <vdi-id> --yes`.

### Step 5 — verify

```sh
xo vm vdis 550e8400-e29b-41d4-a716-446655440001
```

```
ID                                    NAME         TYPE    SIZE     USAGE    SR
------------------------------------  -----------  ------  -------  -------  -------------
11111111-1111-4111-8111-111111111111  system disk  system  10.74GB  5.369GB  Local storage
44444444-4444-4444-8444-444444444444  data-01      user    10.74GB  0B       NFS data
```

Or inspect the attachment directly:

```sh
xo vbd list --vm 550e8400-e29b-41d4-a716-446655440001
```

```
ID                                    VM      VDI          DEVICE  MODE  ATTACHED
------------------------------------  ------  -----------  ------  ----  --------
33333333-3333-4333-8333-333333333333  web-01  system disk  xvda    RW    yes
55555555-5555-4555-8555-555555555555  web-01  data-01      -       RW    no
```

As with every `list` table, the `VM` and `VDI` columns show the resolved
**names** of the referenced objects (not their raw UUIDs); `--output json`
keeps the raw ids.

The new VBD shows the assigned `DEVICE` (e.g. `xvdb`) once attached/hot-plugged.

### Step 6 — prepare the disk inside the guest (Linux)

On the Xen Orchestra side the disk is done; inside the guest it is a plain,
uninitialized block device. Xen Orchestra only manages XenServer / XCP‑ng
pools, so the hypervisor is always Xen and the disk appears as a
paravirtualized device, typically `/dev/xvdb` or `/dev/xvdc` depending on
the devices already present in the guest. The exact name is not guaranteed —
identify the new one by its size rather than by name:

```sh
lsblk
```

```
NAME   MAJ:MIN RM  SIZE RO TYPE MOUNTPOINT
xvda   202:0    0   20G  0 disk
└─xvda1 202:1   0   20G  0 part /
xvdb   202:16   0  10G  0 disk            <- the new disk, still empty
```

Then, from the guest (as root):

```sh
# Format it (no partition table needed for a data disk). This erases
# whatever is on the device — make sure it is really the new disk.
mkfs.ext4 /dev/xvdb

# Mount it now
mkdir /data
mount /dev/xvdb /data

# Mount it at boot (nofail so the boot is not blocked if the disk
# is temporarily missing)
echo "UUID=$(blkid -s UUID -o value /dev/xvdb) /data ext4 defaults,nofail 0 2" >> /etc/fstab
```

If the disk should be managed with LVM instead (volumes that can be grown
later, RAID, …), replace the plain `mkfs` step with:

```sh
pvcreate /dev/xvdb
vgcreate datavg /dev/xvdb
lvcreate -l 100%FREE -n datalv datavg
mkfs.ext4 /dev/datavg/datalv
# then mount /dev/datavg/datalv (or by UUID) as above
```

Notes:

- A **hot-plugged** disk appears in the guest without a reboot (you may need
  to wait a few seconds or trigger a rescan); a disk attached to a halted VM
  is simply present at boot.
- `--shared` VDIs (step 2) allow several VMs to see the same VDI: use a
  cluster-aware filesystem (GFS2/OCFS2) or LVM cluster if they must write to
  it concurrently — a plain ext4 mounted read-write on several guests will be
  corrupted.
- Guest tools may auto-detect new disks (e.g. `lvm2`'s automatic PV
  scanning, cloud-init): if the guest already formatted the disk before you
  did, `mkfs` above would destroy it — check with `lsblk` / `blkid` first.

### Scripting it end to end

Machine-readable output keeps only the requested data on stdout, so the steps
chain cleanly. The VDI id is the only value passed between steps:

```sh
set -euo pipefail
POOL_SR="bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
VM="550e8400-e29b-41d4-a716-446655440001"

# 1. create the disk, keep only its id
VDI_ID=$(xo vdi create data-01 --sr "$POOL_SR" --size 10G --output json | jq -r '.vdi')

# 2. attach it, keep the VBD id
VBD_ID=$(xo vbd create --vm "$VM" --vdi "$VDI_ID" --output json | jq -r '.vbd')

# 3. hot-plug if the VM is running (no-op / refused if halted — check state first)
if [ "$(xo vm get "$VM" --query 'power_state' --output text)" = "Running" ]; then
  xo vbd connect "$VBD_ID"
fi

# 4. verify
xo vm vdis "$VM" --query "[?name_label=='data-01'].id"
```

`--output json` for `vdi create` prints `{"action":"create","vdi":"<id>"}` and
for `vbd create` prints `{"action":"create","vm":"…","vdi":"…","vbd":"<id>"}` —
exactly the fields needed, nothing else.

### Removing the disk (reverse order)

```sh
xo vbd disconnect "$VBD_ID"     # hot-unplug (running VM)
xo vbd delete "$VBD_ID" --yes   # detach (the VDI is kept)
xo vdi delete "$VDI_ID" --yes   # delete the disk (irreversible — needs --yes)
```

The order matters: detach the VBD before deleting the VDI, and a VDI that is
still attached to any VM cannot be deleted.

### Troubleshooting

| Symptom | Likely cause / fix |
| ------- | ------------------ |
| `cannot create VDI: … SR not found` or `invalid --sr id` | Wrong SR UUID. Use `xo sr list`; the SR must be visible to the VM's pool. |
| `cannot attach VDI … to VM …` | The VDI's SR is not available to the VM's pool (local SR on another host, or the VM is on a different pool). Pick a shared SR. |
| Disk created but the guest doesn't see it | The VM is running and you skipped the hot-plug. Run `xo vbd connect <vbd-id>`. |
| `cannot delete VDI` | The VDI is still attached. Run `xo vbd list --vm <id>` and `xo vbd delete` each VBD first. |
| Hot-plug refused | The guest OS or the disk type doesn't support hot-plug; reboot the VM instead. |
| Disk visible on the host (`xo vbd list`) but not in the guest | Check `lsblk` inside the guest: the hot-plug may need a few seconds (or a rescan), or the guest kernel hasn't picked it up — see step 6. |
| `mkfs` / mount fails on the guest | The VBD is not attached (`ATTACHED: no`), the disk is still in use, or you targeted the wrong device — re-check `lsblk` and `xo vbd list --vm <id>`. |

Every failure is reported as `Error: …` on **stderr**; machine-readable output
on stdout stays clean, so scripts can branch on the exit code and the stderr
message.

---

## Create a VM until it is SSH-reachable

Provision a fresh VM from a template and block until the guest is actually
usable — not just powered on. This is the "deploy a server" flow: create and
boot with `xo`, then let `xo vm wait --ssh` be the gate between "the
hypervisor said yes" and "I can log in".

```text
   1. find the pool + template        xo pool list / xo template list
   2. create the VM and boot it       xo vm create --boot --ssh-key
   3. wait until it is ready          xo vm wait --ssh
   4. connect over SSH                ssh user@<ip>   (inside the guest)
   5. verify inside the guest         whoami, systemctl status sshd
```

Steps 1–3 are done with `xo`; step 4 logs in to the guest (the OS is a
black box to Xen Orchestra), step 5 verifies the final state there.

### Prerequisites

- A configured profile (`xo configure`) that can reach the pool.
- The **pool UUID** (`xo pool list`) and a **template id** (`xo template
  list`) belonging to that pool.
- For `--ssh` (step 3): the machine running `xo` must be able to reach the
  guest's IP over TCP — the probe goes from **your machine** to the guest,
  not from the pool's hosts. On a lab where the pool is not on your network,
  wait without `--ssh` and connect through a jump host.
- For key login (step 4): a **public SSH key** to inject at creation
  (`--ssh-key`), and a guest account allowed to use it (the default for
  cloud-init templates).

### Step 1 — find the pool and a template

```sh
xo pool list
```

```
ID                                    NAME     VERSION  CORES  SOCKETS  MASTER   HA
------------------------------------  -------  -------  -----  -------  -------  --
aaaaaaaa-bbbb-cccc-dddd-000000000009  pool-01  8.2.1    32     2        host-01  no
```

```sh
xo template list
```

```
ID                                                                         NAME            DEFAULT  MEMORY   CPUS  POOL
-------------------------------------------------------------------------  --------------  -------  -------  ----  -------
aaaaaaaa-bbbb-cccc-dddd-000000000009-6959dfe8-534c-4c58-8a8c-3c3792293543  Oracle Linux 8  yes      4.295GB  2     pool-01
aaaaaaaa-bbbb-cccc-dddd-000000000009-7aa32be8-a06c-4ade-8a1d-49e51e03e9d2  AlmaLinux 8     no       2.147GB  1     pool-01
```

The `ID` column is the composite template id (`<poolId>-<templateUuid>`);
`vm create` accepts it as-is (or the bare `uuid` field from
`--output json`).

### Step 2 — create the VM, inject the SSH key, and boot it

```sh
xo vm create web-04 --pool aaaaaaaa-bbbb-cccc-dddd-000000000009 \
  --template aaaaaaaa-bbbb-cccc-dddd-000000000009-6959dfe8-534c-4c58-8a8c-3c3792293543 \
  --memory 4G --boot --ssh-key ~/.ssh/id_ed25519.pub
```

```
VM "web-04" created:
  id:     550e8400-e29b-41d4-a716-446655440004
  state:  Starting
  memory: 4.295GB
  cpus:   2
```

`--ssh-key <file>` reads your **public** key from the file and injects it into
the guest with cloud-init (`ssh_authorized_keys`), so step 4 works with key
authentication from the start. This is done **at creation time**:
`cloud_config` is the only path the REST API offers for getting a key into a
guest — there is no action to add one to a VM that already exists — so plan
it in the create call. The flag sends a minimal cloud-init
document; for full control (hostname, packages, extra users, …)
pass a complete user-data file with `--cloud-config <file>` instead — the two
flags are mutually exclusive. The template must support cloud-config (the
standard Xen Orchestra Linux templates do).

`--boot` asks the pool to start the VM as soon as it exists; the re-fetched
state is `Starting` (the start is still in flight), so there is no
"Start it with" hint. Without `--boot`, the VM comes up `Halted` and the
output ends with `Start it with: xo vm start <id>`.

### Step 3 — wait until the guest is ready

```sh
xo vm wait 550e8400-e29b-41d4-a716-446655440004 --ssh --timeout 5m
```

`vm wait` polls the VM every 2 seconds and passes the gate only when **all**
of these hold:

1. the VM is `Running` (a VM that is still `Starting` — the raw
   `power_state` lags at `Halted` while the start task runs — does not pass);
2. it has a **main IP address** (DHCP may take a while after the OS is up);
3. with `--ssh`, port 22 on that IP **accepts a TCP connection** from the
   machine running `xo`.

While the gate is not met the command prints one progress line on stderr and
blocks:

```
Waiting for VM "web-04" to be ready (state is Starting)...
```

When the deadline is reached it fails with what was still missing, so a
script never hangs:

```
Error: VM "web-04" was not ready within 5m0s (port 22 on 10.0.0.14 is not reachable yet)
```

When the guest is ready (exit status 0):

```
VM "web-04" is ready:
  id:     550e8400-e29b-41d4-a716-446655440004
  state:  Running
  ip:     10.0.0.14
  ssh:    reachable on 10.0.0.14:22

Connect with: ssh <user>@10.0.0.14
```

`--timeout` bounds the wait (like `task wait`); without it the wait is
unbounded and Ctrl+C cancels it. The SSH probe checks TCP reachability only,
not the SSH handshake — if sshd or the guest firewall starts late, re-run the
wait or `ssh` (it will retry).

### Step 4 — connect over SSH

```sh
ssh deploy@10.0.0.14
```

### Step 5 — verify inside the guest

```sh
whoami; hostname; systemctl is-active sshd
```

```
deploy
web-04
active
```

### Scripting it end to end

`vm create --output json` prints the full VM object (capture `.id`), and
`vm wait` exits non-zero on a timeout, so the whole flow is a few lines:

```sh
set -euo pipefail
POOL="aaaaaaaa-bbbb-cccc-dddd-000000000009"
TEMPLATE="aaaaaaaa-bbbb-cccc-dddd-000000000009-6959dfe8-534c-4c58-8a8c-3c3792293543"

# 1. create + boot with your public key injected (cloud-init), keep the id
VM_ID=$(xo vm create web-04 --pool "$POOL" --template "$TEMPLATE" --boot \
  --ssh-key ~/.ssh/id_ed25519.pub --output json | jq -r '.id')

# 2. gate: block until the guest answers on :22 (bound it so the script can't hang)
xo vm wait "$VM_ID" --ssh --timeout 10m

# 3. the IP, and in you go
IP=$(xo vm wait "$VM_ID" --ssh --output json --query ip | jq -r .)
ssh "deploy@${IP}"
```

### Troubleshooting

| Symptom | Likely cause / fix |
| ------- | ------------------ |
| `was not ready within … (state is Starting)` | The guest is still booting. Give more time (`--timeout 10m`); check progress with `xo vm get <id>`. |
| `(no main IP address yet)` | DHCP has not assigned an address (or the template has no network). Check the VM's NICs and the pool's default network. |
| `(port 22 on … is not reachable yet)` | sshd starts late, the guest firewall is still closed, the port is not 22 (`--port`), or a network/firewall between you and the guest blocks it. |
| Wait passes but `ssh` is refused | The probe is TCP-only, not an SSH handshake: sshd may not be up yet, or the guest listens on another port. |
| `ssh` authenticates with `publickey` refused | The key was not injected (template without cloud-config support, or the guest user differs from the one cloud-init authorized — check with `--cloud-config`), or you connected with a different key than the `--ssh-key` one. |
| `vm wait` times out on a healthy VM | The probe runs from **your machine**: if you cannot route to the guest IP from where `xo` runs, wait without `--ssh` and connect through a jump host. |
