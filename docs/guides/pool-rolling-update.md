# Pool rolling update and reboot

Apply the available updates to a pool, or reboot all its hosts, one host at a
time so that the pool and its VMs stay available.

```text
   1. inspect the pool and its hosts     xo pool list / get, xo host list
   2. apply the updates                  xo pool rolling-update
   3. (or) reboot every host             xo pool rolling-reboot
   4. follow the task                    xo task list / get / wait
   5. if it was interrupted              check hosts, re-run
```

## Prerequisites

- A profile whose token can manage the pool (see
  [Authentication](../getting-started/authentication.md)).
- Room to evacuate one host: the other hosts must have enough free memory
  for the VMs of the host being rebooted, and the VMs must be on shared
  storage to be live-migrated.
- A maintenance window if some VMs cannot be live-migrated (local disks,
  passed-through devices, no free capacity): they are shut down or suspended
  on their host's turn instead.

## Step 1: inspect the pool

```sh
xo pool list
xo pool get <pool-id>
```

`xo pool list` has the columns `ID NAME VERSION CORES SOCKETS MASTER HA`;
`xo pool get` shows the detail of one pool. List the hosts of the pool and
their state before you start:

```sh
xo host list --query "[?\"\$pool\"=='<pool-id>'].{name: name_label, state: power_state, version: version}"
```

Note the double-quoted `"$pool"`: field names starting with `$` must be
quoted in JMESPath (here escaped for the shell).

## Step 2: apply the updates

```sh
xo pool rolling-update <pool-id> --wait-timeout 2h   # add --yes in a script
```

What happens:

1. Xen Orchestra picks the hosts one by one.
2. The VMs running on the host are moved away (live migration when possible,
   otherwise a scheduled downtime for those VMs).
3. The host is updated and rebooted, and rejoins the pool before the next
   one starts.

`xo` prints `Running rolling update on pool "<name>", this may take a
while…` on stderr, then blocks until the backing task finishes and prints
`Pool "<name>" rolling update applied` (`--output json` prints
`{"action": "rolling update", "pool": "<name>", "status": "done"}`).

Two timeouts are involved:

- `--wait-timeout` bounds the **whole** wait. Without it, the command waits
  until the task completes or you press Ctrl+C. A rolling update of a large
  pool can legitimately take hours.
- The global `--timeout` (default `30s`) bounds **each HTTP request**, that
  is each poll, not the whole operation. You do not need to raise it for a
  long update.

Like `rolling-reboot`, `xo pool rolling-update` asks for confirmation
(`Are you sure you want to rolling update pool "<name>"? [y/N]`). In a
script or without a terminal, pass `--yes` (or set `XOA_YES=1`); otherwise
the command stops with `confirmation required: re-run with --yes to proceed
non-interactively`.

## Step 3 (alternative): reboot every host

```sh
xo pool rolling-reboot <pool-id> --wait-timeout 2h
```

Same host-by-host process, without installing updates. It is considered
destructive, so it asks `Are you sure you want to rolling reboot pool
"<name>"? [y/N]`; pass `--yes` (or `XOA_YES=1`) in scripts. VMs that cannot be
live-migrated away suffer a brief downtime on their host's turn.

## Step 4: follow the task

From another terminal:

```sh
xo task list --status pending
xo task get <task-id>
xo task wait <task-id> --wait-timeout 2h
```

`xo task list` has the columns `ID STATUS TYPE NAME STARTED ENDED`. Watch the
hosts reboot one after the other with `xo host list`.

## Step 5: if the wait was interrupted

Ctrl+C or a reached `--wait-timeout` only stops **the local wait**; the
operation keeps running in Xen Orchestra. In the deadline case `xo` says so:
`rolling update on pool "<name>" did not complete within 2h0m0s (raise
--wait-timeout, or track the task with 'xo task wait')`.

1. Find the task with `xo task list --status pending` and follow it with
   `xo task wait <task-id>`.
2. If you really need to stop it, `xo task abort <task-id>` requests an
   interruption. Hosts already done stay updated; the host in progress may be
   left in maintenance. Check `xo host list` and finish the job from the Xen
   Orchestra web UI if needed.
3. If the task **failed**, read its error with `xo task get <task-id>`, fix the
   cause (often a VM that cannot be evacuated), then run the command again.

## Last resort: emergency shutdown

```sh
xo pool emergency-shutdown <pool-id> --yes
```

**This is not a maintenance tool.** It powers off every host of the pool at
once, without evacuating anything: every running VM goes down hard, the whole
pool is unavailable afterwards, and the hosts must be restarted manually. Use
it only when the pool is in a bad state that nothing else can handle.

## Scripted version

```sh
set -euo pipefail
POOL=$(xo pool list --query "[?name_label=='pool-01'].id | [0]" --output text)

xo pool rolling-update "$POOL" --yes --wait-timeout 3h --output json
xo host list --query "[?\"\$pool\"=='$POOL'].{name: name_label, state: power_state, version: version}"
```

A non-zero exit means the update failed or did not finish within the
deadline; the `Error: ...` line on stderr says which.

## Troubleshooting

| Symptom | Likely cause / fix |
| ------- | ------------------ |
| `invalid pool id "..." (expected a UUID)` | Pass the pool UUID from `xo pool list`, not its name. |
| `pool "..." not found` | Wrong UUID, or the profile points at another Xen Orchestra instance. |
| `... did not complete within ... (raise --wait-timeout, or track the task with 'xo task wait')` | The deadline was reached; the operation is still running. See step 5. |
| `cannot rolling update pool "...": ...` | The operation failed, usually because a host could not be evacuated. Add `--debug` for the API detail. |
| `confirmation required: re-run with --yes to proceed non-interactively` | `rolling-reboot` / `emergency-shutdown` run from a script: add `--yes`. |
| `the request timed out after 30s (raise it with --timeout)` | One poll did not answer in time (network or a busy server). Raise `--timeout`; it does not bound the whole operation. |
