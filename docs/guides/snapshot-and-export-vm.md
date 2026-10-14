# Snapshot and export a VM

Take a point-in-time snapshot of a VM before a risky change, export the VM to
an archive file, and import that archive into another pool (or the same one)
to restore or clone it.

```text
   1. snapshot the VM                 xo vm snapshot
   2. find its snapshots              xo vm get --query snapshots / xo rest
   3. export the VM to a file         xo vm export
   4. import the file into a pool     xo vm import
```

A snapshot lives next to the VM, on the same storage: it protects against a
bad change, not against losing the pool. An export is a self-contained file
you can store anywhere and import into any pool.

## Prerequisites

- A configured profile (see [Configuration](../getting-started/configuration.md)).
- The VM's UUID (`xo vm list`).
- For the import: the target pool's UUID (`xo pool list`) and, optionally, an
  SR of that pool (`xo sr list`).
- Enough local disk space for the archive, and a `--timeout` large enough for
  the transfer (see step 3).

## Step 1: take a snapshot

```sh
xo vm snapshot <vm-id> --name before-upgrade --wait
```

`--name` sets the snapshot label (Xen Orchestra generates one otherwise).
Without `--wait`, the command prints the task id and returns; with it, it
waits for the task and prints the finished task.

## Step 2: find the snapshots of a VM

There is no typed `xo` command to list snapshots yet. The VM object carries
the ids of its snapshots:

```sh
xo vm get <vm-id> --query snapshots --output json
```

To read one snapshot (its name, date, ...), use the raw REST escape hatch.
Xen Orchestra exposes snapshots in the `vm-snapshots` REST collection (check
the exact path in your version's OpenAPI spec):

```sh
xo rest get vm-snapshots/<snapshot-id> --output json
xo rest get vm-snapshots --param fields=id,name_label --output json
```

The fields of a snapshot are listed in the OpenAPI spec at
`<endpoint>/rest/v0/docs`. Deleting or reverting a snapshot has no typed
command either; do it from the Xen Orchestra web UI or through `xo rest`
after checking the endpoint in the OpenAPI spec.

## Step 3: export the VM to a file

```sh
xo vm export <vm-id> --file web-01.xva --timeout 2h
```

- `--format xva` (default) is the native XenServer/XCP-ng format;
  `--format ova` produces an OVA archive for other hypervisors.
- `--compress` is on by default for XVA; `--compress=false` produces a larger
  file faster. It has no effect on OVA.
- Without `--file` (or with `--file -`) the archive is streamed to stdout, so
  it can be piped: `xo vm export <vm-id> > web-01.xva`. The summary line then
  goes to stderr so the stream stays clean.

**Raise the timeout for real VMs.** The export is a single HTTP request, and
the global `--timeout` (default `30s`) bounds the whole transfer. A
multi-gigabyte archive will fail with `the request timed out after 30s (raise
it with --timeout)` unless you raise it with `--timeout` or `$XOA_TIMEOUT`
(for example `2h`).

When it finishes, the command prints the VM name, the destination and the
number of bytes written (`--output json` gives `vm`, `destination` and
`bytes`).

## Step 4: import the archive into a pool

```sh
xo vm import web-01.xva --pool <pool-id> --sr <sr-id> --timeout 2h
```

- `--pool` is required; `--sr` pins the VM's disks to an SR of that pool
  (default: the pool's default SR).
- Only XVA archives are imported. `-` reads the archive from stdin, so an
  export can be piped straight into another pool:

```sh
xo vm export <vm-id> --timeout 2h | xo vm import - --pool <pool-id> --timeout 2h
```

The upload is also one HTTP request bounded by `--timeout`. The command prints
the new VM's id (`--output json` gives `{"vm": "<id>"}`). The imported VM is a
new, independent VM.

## Undo

- Delete an imported copy: `xo vm delete <new-vm-id> --yes`.
- Delete the archive file with `rm`.
- Remove a snapshot from the Xen Orchestra web UI (no typed command yet).

## Scripted version

```sh
set -euo pipefail
VM="550e8400-e29b-41d4-a716-446655440001"
TARGET_POOL="6b7c8d9e-0000-1111-2222-333344445555"
export XOA_TIMEOUT=2h   # bounds each export/import transfer

xo vm snapshot "$VM" --name "nightly-$(date +%F)" --wait --output json > /dev/null
xo vm export "$VM" --file "backup-$(date +%F).xva"
NEW_VM=$(xo vm import "backup-$(date +%F).xva" --pool "$TARGET_POOL" --output json | jq -r '.vm')
echo "restored as $NEW_VM"
```

## Troubleshooting

| Symptom | Likely cause / fix |
| ------- | ------------------ |
| `... the request timed out after 30s (raise it with --timeout)` | The archive transfer is longer than the HTTP timeout. Re-run with `--timeout 2h` (or `XOA_TIMEOUT=2h`). |
| `invalid --format "..." (expected xva or ova)` | Only `xva` and `ova` are supported. |
| `--pool is required (see 'xo pool list')` | `xo vm import` needs the target pool UUID. |
| `"web-01.xva" is empty: the XVA archive cannot be empty` | A failed export leaves the `--file` it created behind, empty or truncated. Check the export's exit code before importing, and delete the file after a failure. |
| `import response did not include the new VM id` | The server accepted the upload but did not report the VM. Look for it with `xo vm list` and check `xo task list`. |
| Import of an `.ova` fails | `xo vm import` only takes XVA archives; export with the default `--format xva`. |
