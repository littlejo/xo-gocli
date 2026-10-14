# Configuration

`xo` reads its connection settings from a **profile** in a configuration
file, and every setting can be overridden from the environment.

- [Configuration file and profiles](#configuration-file-and-profiles)
- [`xo configure`](#xo-configure)
- [Environment variables](#environment-variables)
- [Global flags](#global-flags)
- [Insecure mode](#insecure-mode)
- [Debug mode](#debug-mode)
- [Timeouts and waiting](#timeouts-and-waiting)

## Configuration file and profiles

Profiles are stored in `~/.config/xo/config` (override the location with
`$XOA_CONFIG_FILE`). The file is written with `0600` permissions because it may
hold credentials.

```yaml
current: lab
profiles:
  - name: lab
    endpoint: https://xo.example.com
    token: <token>
    # OR: username: admin / password: <secret>
    # OR: insecure: true
    # Default output format for this profile (table, json, yaml or text):
    # output: json
```

The active profile is resolved in this order: `--profile`, then `$XOA_PROFILE`,
then the file's `current` entry, then the `default` profile.

```sh
xo vm list --profile lab        # one command
export XOA_PROFILE=lab          # every command from this shell
xo vm list
```

Either a token, or a username + password, must be available to authenticate;
see [Authentication](authentication.md) to get a token.

## `xo configure`

Manage configuration profiles: store or update one, and list, inspect or
remove the stored profiles.

```sh
# Store or update a profile (--profile, else $XOA_PROFILE, else the current one)
xo configure                                   # interactive
xo configure --profile lab                     # interactive, named profile
xo configure --profile lab --endpoint https://xo.example.com --token <token>
xo configure --profile lab --username admin --password <secret>
xo configure --profile lab -k                  # skip TLS verification (--insecure)
xo configure --profile lab --token <new-token> # rotate the token, keep the rest
xo configure use lab                           # make lab the current profile

# Inspect and manage the stored profiles
xo configure list                              # all profiles (secrets masked)
xo configure list --output json
xo configure list --query '[].name'            # profile names only
xo configure show lab                          # one profile, full secrets
xo configure show lab --query endpoint         # one field
xo configure remove lab                        # destructive: asks unless --yes
```

Values given with flags win over environment variables; unset values fall back
to the environment (`XOA_ENDPOINT`, `XOA_TOKEN`, `XOA_USERNAME`, `XOA_PASSWORD`),
then to the stored profile, then to an interactive prompt when stdin is a
terminal. An update only changes what you give: the other stored values
(`--insecure` and the `output` setting included) are kept. A token replaces
a stored username and password, and the reverse; `--insecure=false` turns TLS
verification back on.

The first profile you store becomes the current one. Storing another profile
does not change the current profile: make it current with
`xo configure use <name>`, or select it per command.

## Environment variables

Environment variables always take precedence over the stored profile at run
time:

| Variable         | Purpose                                       |
| ---------------- | --------------------------------------------- |
| `XOA_PROFILE`     | Select the active profile                     |
| `XOA_ENDPOINT`    | Xen Orchestra base URL                        |
| `XOA_TOKEN`       | Authentication token                          |
| `XOA_USERNAME`    | Username (alternative to a token)             |
| `XOA_PASSWORD`    | Password (alternative to a token)             |
| `XOA_INSECURE`    | Skip TLS certificate verification             |
| `XOA_TIMEOUT`     | HTTP client timeout, e.g. `60s` or `2m` (like `--timeout`) |
| `XOA_DEFAULT_OUTPUT` | Default output format: `table`, `json`, `yaml` or `text` (like `--output`) |
| `XOA_YES`         | Skip confirmation prompts (like `--yes`)      |
| `XOA_WAIT`        | Wait for async action tasks to complete (like `--wait`) |
| `XOA_DEBUG`       | Show SDK/API error details (like `--debug`)   |
| `XOA_CONFIG_FILE` | Location of the configuration file            |

Either a token, or a username + password, must be available to authenticate.
Token profiles are recommended (see
[Token or password?](authentication.md#token-or-password)). The `XOA_*`
overrides are what CI uses: no token on the command line, and no
configuration file needed.

`XOA_YES` is meant for scripts and CI: `XOA_YES=1 xo vm delete <id>` behaves
like `xo vm delete <id> --yes` without having to pass the flag everywhere.
Likewise `XOA_WAIT=1` behaves like `--wait` on the asynchronous actions (the
`xo vm`, `xo vbd`, `xo pbd` and `xo sr` actions, see the
[command guide](../reference/commands.md)): `XOA_WAIT=1 xo vm start <id>`
waits for the start task to complete without having to pass the flag on every
command.

`XOA_DEFAULT_OUTPUT` is the script counterpart of `--output`:
`XOA_DEFAULT_OUTPUT=json xo vm list` emits JSON without the flag, and an
explicit `--output` still wins for a single command. It is also stored per
profile in the configuration file as `output:` (see
[Choosing the default format](../reference/output-and-query.md#choosing-the-default-format)).

## Global flags

These flags are accepted by every command:

| Flag             | Description                                             |
| ---------------- | ------------------------------------------------------- |
| `-p`, `--profile`| Configuration profile to use (or `$XOA_PROFILE`)         |
| `-o`, `--output` | Output format: `table` (default), `json`, `yaml`, `text` (or `$XOA_DEFAULT_OUTPUT`, or the profile's `output`) |
| `--json`         | Output as JSON, a shortcut for `--output json`          |
| `-d`, `--debug`  | Show SDK/API error details on failure (or `$XOA_DEBUG`) |
| `--timeout`      | HTTP client timeout, e.g. `60s` or `2m` (default `30s`, or `$XOA_TIMEOUT`) |
| `--version`      | Print the CLI version and exit                          |

Commands that return data also accept `--query` / `-q`
(see [Output and querying](../reference/output-and-query.md)). Every command
accepts `--help`.

## Insecure mode

For self-signed or internally-issued certificates:

```sh
xo configure --profile lab --insecure          # stored in the profile
XOA_INSECURE=1 xo vm list                        # per invocation
```

This disables certificate verification and is only appropriate for internal,
trusted networks. When a connection fails on certificate verification and
insecure mode is not enabled, the error points at this escape hatch.

## Debug mode

Errors are intentionally concise: a failed lookup is reported as
`Error: host "…" not found` rather than the raw API response, and common
failure causes are translated from the SDK/Go internals into a single clear
sentence — e.g. `the request timed out after 30s (raise it with --timeout)`,
`cannot reach 127.0.0.1:8443: connection refused`, `the Xen Orchestra API
returned a malformed response`, or `authentication was rejected by the server
(401)`. The global `-d`, `--debug` flag (or the `$XOA_DEBUG` environment
variable) reveals the details behind any of those messages — which profile
and endpoint resolved, and the raw SDK/API error. The diagnostics go to
**stderr**, so machine-readable stdout stays clean even when debugging.

```sh
xo vm get <id> --debug
XOA_DEBUG=1 xo vm get <id>
```

```text
debug: profile=lab endpoint=https://xoa.example.com
debug: API error: 404 Not Found - {"message": "object not found"}
Error: VM "550e8400-…" not found
```

Use it when a failure is hard to explain (a 404 you expected to succeed, a
weird API body, the wrong endpoint reached) and file the output upstream.

## Timeouts and waiting

There are two timeouts, and they bound different things.

**HTTP client timeout** (the global `--timeout`, or `$XOA_TIMEOUT`) — how
long a single HTTP request may take before it is cut off with a `timeout`
error. Defaults to **30 seconds** (the SDK's own default); the flag wins over
the environment variable. For a one-shot
operation (a `list`, a `get`, a single action, a VM/VDI `export` or `import`
stream) this bounds the whole operation, because the operation is one
request. Raise it for a large archive over a slow link:

```sh
XOA_TIMEOUT=5m xo vm export <id> --file vm.xva      # large XVA over a slow link
```

**Wait deadline** — for operations that *poll* a backing task until it
completes (the pool `rolling-update` / `rolling-reboot` / `emergency-shutdown`,
`task wait`, the `--wait` flag of the asynchronous actions, and `vm create` /
`network create` / `create-internal` / `create-bonded`, which always wait for
their creation task), the HTTP
client timeout only bounds each individual poll request, **not** how long the
command blocks overall. By default that overall wait is **unbounded**: the
command blocks until the task completes (or Ctrl+C). The flag that *does*
bound it is `--wait-timeout`, on `task wait` and the three pool maintenance
commands. The global `--timeout` keeps its meaning there (each poll request):

```sh
xo pool rolling-update <id> --wait-timeout 30m   # give up on the wait after 30 min
xo task wait <id> --wait-timeout 5m              # …but give up after 5 minutes
xo task wait <id> --wait-timeout 1h --timeout 2m # long wait, slow polls allowed
```

There is **no default wait deadline**: a pool update, a stuck task or a large
import can legitimately run for a long time, and a fixed cutoff would report
a timeout even when the operation is still progressing (or has completed)
server-side — leaving the caller unsure of the real state. Instead the wait
ends when the task completes or you press Ctrl+C (which propagates from the
CLI through the SDK to the HTTP layer); use `--wait-timeout` above when
you want a bound anyway. The wait deadline and the per-request HTTP timeout
are independent: a 30-minute wait is made of many 30-second-bounded polls, so
a broken connection can never hang a single poll forever.

`vm create` and `network create*` have no `--wait-timeout`: only Ctrl+C ends
their wait early. They wait through the SDK, which does not yet treat an
`interrupted` task as finished
([vatesfr/xenorchestra-go-sdk#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121)),
so if their creation task is interrupted (for example with `xo task abort`)
they keep waiting until you press Ctrl+C. The `--wait` flag of the
asynchronous actions has no deadline either; to bound one, run the action
without `--wait` and pass the printed task id to
`xo task wait <task-id> --wait-timeout <duration>`.
