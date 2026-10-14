# Troubleshooting

Every failure is reported as a single `Error: ...` line on **stderr**, and
`xo` exits with code `1` (see [Exit codes](reference/exit-codes.md)). The
message is kept short on purpose; add `--debug` (or `XOA_DEBUG=1`) to also
print the profile and endpoint that were used and the raw API error behind
the message:

```sh
xo vm list --debug
```

```text
debug: profile=default endpoint=http://127.0.0.1:1
debug: cannot list VMs: failed to do request Get "http://127.0.0.1:1/rest/v0/vms?fields=%2A": dial tcp 127.0.0.1:1: connect: connection refused
Error: cannot list VMs: cannot reach 127.0.0.1:1: connection refused
```

In the tables below, `...` stands for the part of the message that depends
on the command (`cannot list VMs: `, `cannot get pool "<id>": `, ...).

## Connection

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| `... cannot reach <host>:<port>: connection refused` | Nothing listens at the endpoint: wrong host or port, or Xen Orchestra is down. | Check the endpoint with `xo configure show`, and that `XOA_ENDPOINT` is not overriding it. |
| `... cannot reach <host>:<port>: i/o timeout` | A firewall drops the traffic, or the host is unreachable from here. | Check the network path to the endpoint. |
| `... tls: failed to verify certificate: x509: ...` followed by `the server certificate could not be verified; if this is a self-signed or internal certificate, retry with 'xo configure --insecure' (or set XOA_INSECURE=1)` | The server uses a self-signed or internal certificate. | Store the profile again with `--insecure` (`xo configure --profile <name> --endpoint <url> --token <token> --insecure`), or set `XOA_INSECURE=1` for one run. Prefer installing the CA on the machine for production. |
| `... the request timed out after 30s (raise it with --timeout)` | One HTTP request took longer than the client timeout (default `30s`). Typical for `xo vm export` / `xo vm import` of a large archive, or a slow server. | Raise it: `--timeout 5m` or `XOA_TIMEOUT=5m`. Exports and imports are one request each: give them the time of the whole transfer. |
| `... the Xen Orchestra API returned a malformed response` | The endpoint answers, but not with the Xen Orchestra REST API (wrong URL, a proxy or login page in front of it). | Use the base URL of Xen Orchestra (e.g. `https://xo.example.com`), without `/rest/v0`. |

## Authentication

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| `... authentication was rejected by the server (401)` | The token is wrong, expired or revoked (or the username/password is wrong). | Create a new token (see [Authentication](getting-started/authentication.md)) and store it: `xo configure --profile <name> --endpoint <url> --token <new-token>`. Check that `XOA_TOKEN` is not set to an old value. |
| `... the server rejected the request (403): the credentials lack the required permission` | The user behind the token lacks the permission for this action. | Use a token of a user with the required rights in Xen Orchestra. |
| `authentication to <endpoint> failed: ...` | Login with `--username` / `--password` failed before any request. | Check the credentials, or switch to a token. |

## Configuration

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| `no endpoint configured for profile "default", set the XOA_ENDPOINT environment variable or run 'xo configure'` | No profile is stored and `XOA_ENDPOINT` is not set. | Run `xo configure`, or export `XOA_ENDPOINT` and `XOA_TOKEN` (see [Configuration](getting-started/configuration.md)). |
| `no credentials configured for profile "<name>", run 'xo configure' or set XOA_TOKEN (or XOA_USERNAME and XOA_PASSWORD)` | The endpoint is known but there is no token or username/password. | Store a token with `xo configure`, or export `XOA_TOKEN`. |
| `profile "<name>" is not configured, run 'xo configure --profile <name>'` | `--profile` or `XOA_PROFILE` names a profile that is not stored. This is an error even when `XOA_ENDPOINT` / `XOA_TOKEN` are set. | Check `xo configure list` and `echo $XOA_PROFILE`; create the profile or unset the variable. |
| Commands reach the wrong Xen Orchestra, or use the wrong token | `XOA_ENDPOINT`, `XOA_TOKEN`, `XOA_INSECURE`, ... override the values of the selected profile. | Run `env \| grep ^XOA_` and unset what you did not mean to set. `--debug` shows the profile and endpoint actually used. |
| The current profile changed | `xo configure --profile <name>` always makes `<name>` the current profile. | Select profiles explicitly with `--profile` or `XOA_PROFILE` (see [Several pools and profiles](guides/multiple-pools-profiles.md)). |
| A profile selected with `XOA_PROFILE` was not updated by `xo configure`; the `default` profile was | `xo configure` reads its target only from `--profile`. | Use `xo configure --profile lab --endpoint <url> --token <token>`. |
| `invalid endpoint "...": it must start with http:// or https://` | The endpoint was given without a scheme. | Use `https://xo.example.com`. |

## Commands and arguments

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| `unknown command "lst" for "xo vm" (did you mean get, list?), run 'xo vm --help' for the available commands` | Misspelled sub-command. | Use one of the suggestions, or `xo vm --help`. |
| `unknown command "vms" for "xo"` followed by `Did you mean this?` | Misspelled resource group (groups are singular: `vm`, `host`, `pool`, ...). | See [the command list](reference/commands.md). |
| `unknown flag: --bogus` / `unknown shorthand flag: 'v' in -v` | The flag does not exist on that command (for example, there is no `-v`; use `--debug`). | Check the command's `--help`, e.g. `xo vm stop --help`. |
| `invalid VM id "web-01" (expected a UUID)` (or `invalid pool id ...`, `invalid --pool id ...`) | `get` and every action take a **UUID**, not a name. | Find the UUID by name: `xo vm list --query "[?name_label=='web-01'].id \| [0]" --output text`. |
| `VM "<id>" not found` (or `pool`, `task`, ...) | The UUID does not exist on this Xen Orchestra, or it is the id of another kind of object (a template is not a VM). | Check the id with the matching `list` command, and the profile in use. |
| `confirmation required: re-run with --yes to proceed non-interactively` | A destructive command runs without a terminal (script, CI, pipe). | Add `--yes` or set `XOA_YES=1`. |
| `aborted` | You answered anything but `y` / `yes` at the confirmation prompt. | Re-run and confirm, or add `--yes`. |

## Waits and tasks

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| A `--wait` action or `xo task wait` does not return | The task is still `pending` (or stuck). Without `--wait-timeout` the wait is unbounded. | In another terminal: `xo task list --status pending`, `xo task get <task-id>`. Stop the operation with `xo task abort <task-id>` if needed. Bound future waits with `xo task wait <task-id> --wait-timeout 10m`. |
| `task <id> did not complete within 10m0s` | The `--wait-timeout` deadline of `xo task wait` was reached. The task keeps running. | Wait again, or abort it with `xo task abort <task-id>`. |
| `task <id> failed: <message>` / `task <id> was interrupted` | The operation failed or was aborted server-side. | Read the full task with `xo task get <task-id> --output json`. |
| `xo vm create`, `xo network create`, `create-internal` or `create-bonded` never returns | These commands wait for their creation task **without a deadline**, and have no `--wait-timeout` yet (pending SDK issue [vatesfr/xenorchestra-go-sdk#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121)). | Only Ctrl+C ends the wait. Then check the task with `xo task list --status pending` and the object with `xo vm list` / `xo network list`. |
| `<action> on pool "<name>" did not complete within ... (raise --wait-timeout, or track the task with 'xo task wait')` | The `--wait-timeout` of a pool maintenance command was reached. The operation keeps running. | Follow it with `xo task list --status pending` and `xo task wait <task-id>` (see [Pool rolling update](guides/pool-rolling-update.md)). |
| A long wait fails with `the request timed out after 30s (raise it with --timeout)` | The global `--timeout` bounds **each request** (each poll), not the wait. One poll was too slow. | Raise `--timeout` / `XOA_TIMEOUT`; use `--wait-timeout` for the total duration. |

## Output

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| `jq` cannot parse the output | The command printed a table (the default). | Add `--output json` (or `--json`). |
| Commands print JSON or YAML instead of a table | `XOA_DEFAULT_OUTPUT` is set, or the profile has an `output` value. | `unset XOA_DEFAULT_OUTPUT`, or pass `--output table`. The `--output` flag always wins. |
| `unsupported --output format "xml" (expected one of: table, json, yaml, text)` | Unknown format. | Use `table`, `json`, `yaml` or `text`. |
| `invalid --query expression "...": ...` | JMESPath syntax error, a missing field passed to a function, or a `$` field not double-quoted. | See [Bulk actions with --query](guides/bulk-actions-with-query.md) and [Output and query](reference/output-and-query.md). |
| A `--query` filter returns nothing | Field names and values are case-sensitive; the table headers are not the JSON field names. | Look at one object with `--output json` first. |

## Reporting a bug

1. Check that you run the latest release: `xo version`.
2. Re-run the failing command with `--debug` and keep the full stderr.
3. Open an issue at <https://github.com/littlejo/xo-gocli/issues> with the
   command, the `xo version` output, the debug output, and what you expected.
   **Remove tokens, passwords and internal host names** from everything you
   paste (command lines, environment, debug output) first.

Security issues must not be reported in a public issue: follow
[SECURITY.md](../SECURITY.md).
