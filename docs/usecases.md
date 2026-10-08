# Use cases

Task-oriented recipes that put several `xo` commands together to achieve
something real. Each use case shows the commands in order, the expected output,
and how to script it. For the full command reference see
[usage.md](usage.md); for the architecture see [development.md](development.md).

A use case is *complete* when every step is a typed `xo` command (no `xo rest`
needed) and the final state is verified.

## Table of contents

- [Add a disk to a VM](#add-a-disk-to-a-vm)
- [Turn a downloaded image into a VM template](#turn-a-downloaded-image-into-a-vm-template)

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

## Turn a downloaded image into a VM template

You have downloaded an image of a machine you want to run repeatedly — for
example a Linux node with **Tailscale** installed — and you want it to be
available in Xen Orchestra as a **VM template**, so that new nodes are a one
line away: `xo vm create node-01 --pool … --template …`.

This is the recipe to go from a file on your laptop to a template in `xo
template list`, and to the VMs created from it.

```text
   1. identify the image format          file
   2. find the target pool and SR        xo pool list / xo sr list
   3. get the image into a VM            xo vm import (XVA/OVA)
                                         xo vm create + xo vdi import (raw disk)
   4. boot it and verify it works        xo vm start --wait
   5. shut it down cleanly               xo vm stop --wait
   6. convert VM -> template             web UI (REST gap — see step 6)
   7. verify the template                xo template list / get
   8. create VMs from it                 xo vm create --template
```

### Why "import a template" is not one command

Two things about Xen Orchestra make this a multi-step flow:

- **A template is a VM, not a file.** A "template" is simply a VM with an
  `is_a_template` flag. There is no template file format to upload, and the
  REST API has no "import template" endpoint: importing always produces a
  plain **VM** first.
- **XO imports whole VMs, not bare disks.** `xo vm import` (REST
  `POST /pools/<pool>/vms`) accepts **XVA or OVA** archives — a full
  virtual machine, disks included. A bare disk image (qcow2, raw, VHD) has to
  go *into* an existing VM's disk, with `xo vdi import`.
- **The VM → template conversion is not in the REST API** (step 6). The only
  programmatic route is the legacy JSON-RPC `vm.convertToTemplate`, which this
  CLI does not call; the modern REST API and the Go SDK v2 have no equivalent
  yet. Until then, the conversion is the **web UI** button (and a documented
  upstream gap).

So: import as a VM, make sure it boots, then flip it to a template. The
conversion is **one-way** — afterwards the object no longer appears in
`xo vm list`, only in `xo template list`.

### Prerequisites

- A configured profile (`xo configure`) with **admin** rights on the pool
  (importing needs `import:vm` on the SR, the conversion needs
  `administrate` on the pool).
- A shared **storage repository (SR)** the new VMs will use — `xo sr list`.
- The image file. If it is a **qcow2** or VirtualBox **vdi**, a machine with
  `qemu-img` (the `qemu-utils` package) to convert it, because XO disk import
  accepts **raw and vhd only**.

### Step 1 — identify the image

```sh
file tailscale.qcow2
```

```
tailscale.qcow2: QEMU QCOW2 Image, version 3, 8589934592 bytes
```

Route the result:

| `file` says (or the extension) | It is | Path |
| ------------------------------ | ----- | ---- |
| XVA (`gzip compressed data…`, `.xva`) or OVA (`POSIX tar archive`, `.ova`) | A **full VM export** | Short path — [step 3a](#step-3a-short-path-xva--ova) |
| `QEMU QCOW2 Image`, `data` (`.raw`/`.img`), `Microsoft Virtual PC Hard Disk` (`.vhd`), VirtualBox `.vdi` | A **bare disk** | Long path — [step 3b](#step-3b-long-path-bare-disk-the-tailscale-case) |

Convert a qcow2 (or vdi) to a format XO understands, once:

```sh
qemu-img convert -f qcow2 -O raw tailscale.qcow2 tailscale.raw
```

A raw disk is a *guest OS on a disk* — no boot order, no network card, no
name. The long path wraps it in a VM that provides the rest.

### Step 2 — find the target pool and SR

```sh
xo pool list
```

```
ID                                    NAME     VERSION  CORES  SOCKETS  MASTER   HA
------------------------------------  -------  -------  -----  -------  -------  --
aaaaaaaa-bbbb-cccc-dddd-000000000001  pool-01  8.2.1    16     2        host-01  no
```

```sh
xo sr list
```

```
ID                                    NAME           TYPE  SIZE   USAGE  CONTAINER
------------------------------------  -------------  ----  -----  -----  ---------
aaaaaaaa-bbbb-cccc-dddd-000000000001  Local storage  lvm   200GB  95GB   pool-01
bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb  NFS data       nfs   2TB    1.2TB  pool-01
```

Pick the pool (e.g. `aaaaaaaa-…`) and, for the disk path, a **shared** SR
(e.g. `bbbbbbbb-…` — the VM's disks must live where the pool can see them).

### Step 3a — short path: XVA / OVA

The archive already *is* a VM, so one command imports it:

```sh
xo vm import tailscale.ova --pool aaaaaaaa-bbbb-cccc-dddd-000000000001
```

```
Imported VM 9fe12ca3-d75d-cfb0-492e-cfd2bc6c568f
```

Pin the disks to a specific SR with `--sr` (default: the pool's default SR).
The upload is one single HTTP request, so the global timeout bounds it — for a
large archive over a slow link, raise it: `--timeout 30m` or `$XOA_TIMEOUT`.

Go to [step 4](#step-4--boot-it-and-verify-it-works).

### Step 3b — long path: bare disk (the Tailscale case)

Wrap the disk in a throwaway VM built from a **base template** of the pool —
any generic template works (the stock pool template `other_install` is the
usual choice); the VM it produces supplies the boot loader slot, the network
card and the shell.

1. **Create the shell VM.** The base template decides the system disk size —
   it must be at least as big as the image you are about to write into it
   (a VDI can only be grown, never shrunk — see the troubleshooting table at
   the end of this use case).

   ```sh
   xo vm create tailscale-base --pool aaaaaaaa-bbbb-cccc-dddd-000000000001 \
     --template aaaaaaaa-bbbb-cccc-dddd-000000000001-3f9b5e2a-7c1d-4e8f-9a0b-1c2d3e4f5a6b \
     --memory 2G
   ```

   ```
   VM "tailscale-base" created:
     id:     66666666-6666-4666-8666-666666666666
     state:  Halted
     memory: 2.147GB
     cpus:   1

   Start it with: xo vm start 66666666-6666-4666-8666-666666666666
   ```

   `--template` accepts the composite id printed by `xo template list`
   (`<poolId>-<templateUuid>`) or the bare template UUID.

2. **Find the system disk** the image will replace (the VM is halted, so its
   disks are safe to write):

   ```sh
   xo vm vdis 66666666-6666-4666-8666-666666666666
   ```

   ```
   ID                                    NAME         TYPE    SIZE     USAGE  SR
   ------------------------------------  -----------  ------  -------  -----  --------
   22222222-2222-4222-8222-222222222222  system disk  system  10.74GB  0B     NFS data
   ```

3. **Write the image into it.** This **overwrites** the disk's content, so it
   asks for confirmation (`--yes` for scripts):

   ```sh
   xo vdi import 22222222-2222-4222-8222-222222222222 tailscale.raw --format raw --yes
   ```

   ```
   Imported image into VDI "system disk"
   ```

   Use `--format vhd` for a VHD image. The disk size shown by
   `xo vm vdis` is unchanged (only the content was written) — the guest now
   sees its original filesystem on a disk of the base template's size.

### Step 4 — boot it and verify it works

```sh
xo vm start 66666666-6666-4666-8666-666666666666 --wait
```

```
ID                                    STATUS   TYPE  NAME      STARTED               ENDED                 MESSAGE
------------------------------------  -------  ----  --------  --------------------  --------------------  -------
77777777-7777-4777-8777-777777777777  success  VM    start VM  2026-10-08T09:00:00Z  2026-10-08T09:00:04Z
```

`--wait` blocks until the start task completes and prints it (like `xo task
wait`); the exit status is non-zero if the start failed. Then prove the image
is healthy — this is the only step the CLI cannot do for you: console or SSH
in, check the OS boots, the services start, and (for Tailscale) that the
daemon runs: `systemctl status tailscaled`. Only convert a template you would
be happy to boot blindly.

### Step 5 — shut it down cleanly

A template should be captured in a clean, halted state:

```sh
xo vm stop 66666666-6666-4666-8666-666666666666 --wait
```

```
ID                                    STATUS   TYPE  NAME               STARTED               ENDED                 MESSAGE
------------------------------------  -------  ----  -----------------  --------------------  --------------------  -------
88888888-8888-4888-8888-888888888888  success  VM    clean shutdown VM  2026-10-08T09:05:00Z  2026-10-08T09:05:12Z
```

### Step 6 — convert the VM into a template (the REST gap)

> **This step has no `xo` command — by design, not by oversight.** The XO
> REST API exposes no way to flip a VM into a template: the `vm-templates`
> resource is read/tag/delete/export only, and `PATCH /vms/{id}` accepts
> editable VM fields (name, memory, CPUs, tags, …) but no `isTemplate`. The
> only programmatic conversion is the legacy JSON-RPC `vm.convertToTemplate`,
> which this CLI deliberately does not call (SDK v1 is out of scope here).
> The gap should be contributed to the XO REST API and, with it, to the Go
> SDK v2; until then the web UI does the job.

In the **Xen Orchestra web UI**: open the VM → **Advanced** tab →
**Convert to template** → confirm.

What happens:

- the object keeps its id, name, disks and configuration, and becomes a
  `VM-template`;
- it **disappears from `xo vm list`** and appears in `xo template list` —
  the conversion is one-way, so make sure step 4 really passed;
- it requires the `administrate` permission on the pool.

> Want to keep the running VM as well? Duplicate it first (the REST API has a
> `POST /vms/{id}/actions/clone` action, not yet a typed CLI command — `xo
> rest post vms/<id>/actions/clone --data '{"name_label":"tailscale-keep"}'`),
> convert the duplicate, and keep or delete the original.

### Step 7 — verify the template

```sh
xo template list
```

```
ID                                                                         NAME            DEFAULT  MEMORY   CPUS  POOL
-------------------------------------------------------------------------  --------------  -------  -------  ----  -------
aaaaaaaa-bbbb-cccc-dddd-000000000001-3f9b5e2a-7c1d-4e8f-9a0b-1c2d3e4f5a6b  other_install   no       2.147GB  1     pool-01
aaaaaaaa-bbbb-cccc-dddd-000000000001-66666666-6666-4666-8666-666666666666  tailscale-base  no       2.147GB  1     pool-01
```

```sh
xo template get aaaaaaaa-bbbb-cccc-dddd-000000000001-66666666-6666-4666-8666-666666666666
```

```
Template tailscale-base
Pool:        pool-01
Memory:      2.147GB
CPUs:        1
Power state: Halted
```

The id of a template is the **composite** `<poolId>-<templateUuid>` — for a
template you made by hand, that is the pool id plus the UUID the VM had before
conversion (`$POOL-66666666-…` above).

### Step 8 — create VMs from the template

```sh
xo vm create tailscale-node-01 --pool aaaaaaaa-bbbb-cccc-dddd-000000000001 \
  --template aaaaaaaa-bbbb-cccc-dddd-000000000001-66666666-6666-4666-8666-666666666666
```

```
VM "tailscale-node-01" created:
  id:     88888888-8888-4888-8888-888888888888
  state:  Halted
  memory: 2.147GB
  cpus:   1

Start it with: xo vm start 88888888-8888-4888-8888-888888888888
```

`--template` also accepts the bare template UUID, and `--boot` starts the VM
as soon as it is created. Every VM created this way gets a fresh copy of the
template's disks on the pool's default SR.

### Tailscale-specific notes

A Tailscale image has state that a generic OS image does not:

- **Do not bake in a live node identity.** If the source machine was
  authenticated to a tailnet, its node key is in `/var/lib/tailscale`.
  Clones of the template would all present the *same* identity. Before
  step 6, either wipe the Tailscale state in the guest (e.g.
  `tailscale down --delete-keys && rm -rf /var/lib/tailscale`) or plan to run
  `tailscale up --auth-key=<key>` in every clone after first boot.
- **Hostnames.** Tailscale advertises the machine's hostname; if the guest's
  hostname is baked in, all clones look identical in the tailnet. Change it
  per clone (e.g. a first-boot script driven by the VM's `name_label`).
- **The template's Tailscale config is a starting point** — ACLs, exit-node
  and subnet-router roles are per-node and must be re-decided for each VM.

### Scripting it end to end

Machine-readable output makes the typed steps chainable; the script must
**pause at the conversion** (step 6), which has no REST endpoint yet:

```sh
set -euo pipefail
POOL="aaaaaaaa-bbbb-cccc-dddd-000000000001"
BASE_TEMPLATE="aaaaaaaa-bbbb-cccc-dddd-000000000001-3f9b5e2a-7c1d-4e8f-9a0b-1c2d3e4f5a6b"
IMAGE="tailscale.raw"

# 1. shell VM from the base template (system disk >= image size)
VM_ID=$(xo vm create tailscale-base --pool "$POOL" --template "$BASE_TEMPLATE" \
  --memory 2G --output json | jq -r '.id')

# 2. write the image into its system disk (the VM is halted)
SYS_VDI=$(xo vm vdis "$VM_ID" --output json | jq -r '.[] | select(.VDI_type=="system") | .id')
xo vdi import "$SYS_VDI" "$IMAGE" --format raw --yes

# 3. verify it boots, then capture it halted
xo vm start "$VM_ID" --wait
# … console/SSH: the OS boots, tailscaled runs …
xo vm stop "$VM_ID" --wait

# 4. web UI: VM -> Advanced -> Convert to template. Then:
TEMPLATE_ID="$POOL-$VM_ID"     # the template's REST id, from the VM's UUID
xo template list --query "[?id=='$TEMPLATE_ID'].name_label"

# 5. create the fleet
for i in 01 02 03; do
  xo vm create "tailscale-node-$i" --pool "$POOL" --template "$TEMPLATE_ID"
done
```

`vm create --output json` prints the full VM object (`.id` is the new VM's
UUID); `vm vdis --output json` prints the raw VDI objects (`.id`,
`.VDI_type`); the template's REST id is simply the pool UUID + `-` + the VM
UUID it was converted from.

### Troubleshooting

| Symptom | Likely cause / fix |
| ------- | ------------------ |
| `xo vm import` fails with an API error on a `.qcow2` / `.raw` / `.vhd` file | Bare disks cannot be imported as VMs. `xo vm import` takes XVA/OVA only — use the long path (step 3b). |
| `cannot import into VDI …` / the import is refused | `xo vdi import` accepts **raw and vhd only** (no qcow2, no vdi). Convert first with `qemu-img convert`. |
| The image does not fit the system disk | The VDI must be at least as big as the image, and a VDI cannot be shrunk. Either pick a base template with a bigger system disk, or grow it with the REST escape hatch `xo rest patch vdis/<id> --data '{"size": <bytes>}'` (grow-only, size in bytes) — the CLI has no typed resize command yet. |
| Import (or upload) times out | The whole transfer is one HTTP request bounded by the global timeout (30 s default). Raise it with `--timeout` or `$XOA_TIMEOUT`. |
| The VM still shows in `xo vm list` after the "conversion" | The conversion was not performed (or was cancelled in the UI). It only happens through the web UI's *Convert to template*; there is no CLI/REST command for it yet. |
| New VMs created from the template do not join the tailnet / show as duplicate nodes | The image carried the source machine's Tailscale identity — see the [Tailscale-specific notes](#tailscale-specific-notes). |
| The shell VM booted into a rescue prompt after the import | The image's boot loader expects different disk geometry (e.g. it was captured on a smaller/larger disk). Boot the shell VM from the console, fix the bootloader or partition table inside the guest, then redo steps 5–6. |
| I want the original VM back | The conversion is one-way. Clone before converting (see the note in step 6), or re-import the XVA you exported beforehand (`xo vm export <id> --file backup.xva`). |

Every failure is reported as `Error: …` on **stderr**; machine-readable output
on stdout stays clean, so scripts can branch on the exit code and the stderr
message.
