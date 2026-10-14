# Automate `xo` in CI

Run `xo` from a CI pipeline or a cron job: no configuration file, no prompt,
machine-readable output, and an exit code you can trust.

```text
   1. configure with environment variables only   XOA_ENDPOINT, XOA_TOKEN
   2. ask for machine output                      --output json, --query
   3. never prompt                                --yes / XOA_YES
   4. wait for asynchronous work                  --wait / XOA_WAIT, xo task wait
   5. branch on the exit code                     0 = success, 1 = any error
```

## Prerequisites

- A dedicated token for the pipeline, stored as a CI secret. See
  [Authentication](../getting-started/authentication.md); a token with an
  expiry can be created with
  `xo token create --description "ci pipeline" --expires-in "30 days"`.
- The `xo` binary on the runner (the install script is shown in the
  GitHub Actions example below).

## Step 1: configure with environment variables only

```sh
export XOA_ENDPOINT=https://xo.example.com
export XOA_TOKEN=<token>
xo pool list
```

No configuration file and no `xo configure` are needed: when no file exists,
`xo` uses the `default` profile and fills it from the environment. The
variables also override a stored profile, so the same pipeline works on a
machine that has one. Useful variables:

| Variable | Effect |
| -------- | ------ |
| `XOA_ENDPOINT`, `XOA_TOKEN` | Connection (or `XOA_USERNAME` + `XOA_PASSWORD`). |
| `XOA_INSECURE=1` | Skip TLS verification (self-signed certificate). |
| `XOA_DEFAULT_OUTPUT=json` | Default output format for every command. |
| `XOA_TIMEOUT=2m` | HTTP timeout of each request (default `30s`). |
| `XOA_YES=1` | Answer yes to every confirmation. |
| `XOA_WAIT=1` | Make asynchronous actions wait, like `--wait`. |
| `XOA_DEBUG=1` | Print the raw API error on failure. |

Do not set `XOA_PROFILE` in CI unless that profile exists on the runner: a
missing profile is an error even when `XOA_ENDPOINT` and `XOA_TOKEN` are set.
See [Configuration](../getting-started/configuration.md) for the full
precedence rules.

## Step 2: ask for machine output

`--output json` (or `--json`) prints only the data on stdout; every error goes
to stderr as a single `Error: ...` line. Extract fields with `jq` or with the
built-in JMESPath `--query`:

```sh
xo vm list --output json | jq -r '.[] | select(.power_state == "Running") | .name_label'
xo vm list --power-state Running --query '[].name_label' --output text
```

`--output text` prints a list of scalars one per line and a single value
bare, which is the easiest form for shell variables. See
[Output and query](../reference/output-and-query.md).

## Step 3: never prompt

Destructive commands (`vm delete`, `task abort`, `pool rolling-reboot`, ...)
ask for confirmation. When stdin is not a terminal they refuse instead of
hanging, with `confirmation required: re-run with --yes to proceed
non-interactively`. Pass `--yes` on each command, or set `XOA_YES=1` once for
the job.

## Step 4: wait for asynchronous work

Power actions and snapshots start a task and return immediately. Two ways to
wait:

- `--wait` (or `XOA_WAIT=1`) on the action: it waits for the task and prints
  it; a failed task gives exit code 1.
- `xo task wait` on a task id, with `--wait-timeout` as a deadline, which
  makes it a gate:

```sh
TASK=$(xo vm start <vm-id> --output json | jq -r '.task_id')
xo task wait "$TASK" --wait-timeout 10m --query status --output text
```

`xo task wait` exits 0 only when the task succeeds; a failed or interrupted
task, a reached deadline (`task <id> did not complete within 10m0s`) or an
unknown task gives exit code 1. The global `--timeout` only bounds each poll.

## Step 5: branch on the exit code

`xo` exits with `0` on success and `1` on any error (bad flag, unreachable
endpoint, API error, failed task). There are no other codes; read the stderr
line to tell errors apart. See [Exit codes](../reference/exit-codes.md).

## Example: GitHub Actions job

```yaml
name: nightly-snapshot
on:
  schedule:
    - cron: "0 2 * * *"
  workflow_dispatch:

jobs:
  snapshot:
    runs-on: ubuntu-latest
    env:
      XOA_ENDPOINT: ${{ secrets.XOA_ENDPOINT }}
      XOA_TOKEN: ${{ secrets.XOA_TOKEN }}
      XOA_DEFAULT_OUTPUT: json
      XOA_TIMEOUT: 2m
      XOA_YES: "1"
    defaults:
      run:
        shell: bash
    steps:
      - name: Install xo
        run: |
          mkdir -p "$HOME/.local/bin"
          curl -fsSL https://raw.githubusercontent.com/littlejo/xo-gocli/main/install.sh | XOA_INSTALL_DIR="$HOME/.local/bin" sh
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"

      - name: Check the connection
        run: xo version && xo pool list --query '[].name_label'

      - name: Snapshot every VM tagged "nightly"
        run: |
          ids=$(xo vm list --query "[?contains(tags, 'nightly')].id" --output text)
          for id in $ids; do
            xo vm snapshot "$id" --name "nightly-$(date +%F)" --wait
          done
```

With `shell: bash` set explicitly, GitHub runs each `run:` block with
`bash -e -o pipefail`, so the first failing `xo` command (including one on the
left of a pipe) fails the job. Each snapshot prints the finished task as JSON
because of `XOA_DEFAULT_OUTPUT`.

## Example: plain shell script

```sh
#!/usr/bin/env bash
set -euo pipefail
: "${XOA_ENDPOINT:?set XOA_ENDPOINT}" "${XOA_TOKEN:?set XOA_TOKEN}"

VM="550e8400-e29b-41d4-a716-446655440001"

# Stop, then gate on the task with a deadline
TASK=$(xo vm stop "$VM" --output json | jq -r '.task_id')
if ! xo task wait "$TASK" --wait-timeout 5m --output json > /tmp/stop-task.json; then
  echo "stop failed or timed out, see /tmp/stop-task.json" >&2
  exit 1
fi

STATE=$(xo vm get "$VM" --query power_state --output text)
[ "$STATE" = "Halted" ] || { echo "unexpected state: $STATE" >&2; exit 1; }
```

## Troubleshooting

| Symptom | Likely cause / fix |
| ------- | ------------------ |
| `no endpoint configured for profile "default", set the XOA_ENDPOINT environment variable or run 'xo configure'` | The secret is not exported to the step. Check the `env:` block. |
| `no credentials configured for profile "default", run 'xo configure' or set XOA_TOKEN (or XOA_USERNAME and XOA_PASSWORD)` | Same, for the token. |
| `... authentication was rejected by the server (401)` | The token expired or was revoked. Create a new one and update the secret. |
| `confirmation required: re-run with --yes to proceed non-interactively` | Add `--yes` or `XOA_YES=1`. |
| `jq` fails to parse the output | The command printed a table: add `--output json` or set `XOA_DEFAULT_OUTPUT=json`. |
| `.task_id` is `null` | `XOA_WAIT=1` is set, so the action already waited and printed the task itself; read `.status` instead. |

More in [Troubleshooting](../troubleshooting.md).
