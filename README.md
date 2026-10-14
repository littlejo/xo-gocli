# xo Go CLI

A command line client for [Xen Orchestra](https://github.com/vatesfr/xen-orchestra),
written in Go, with an AWS-CLI-like feel. It is the day-to-day tool for people
who run VMs and the scriptable one for pipelines that need to talk to Xen
Orchestra from the shell.

`xo` is a thin UX layer over the official Go SDK
([`github.com/vatesfr/xenorchestra-go-sdk/v2`](https://github.com/vatesfr/xenorchestra-go-sdk)).
It talks to the Xen Orchestra **REST API** only — there is no second HTTP
client and no legacy JSON-RPC (v1) code path:

```text
   xo Go CLI (commands / output / query / config)
        │
        ▼
   xenorchestra-go-sdk/v2          ← the only Xen Orchestra client
        │
        ▼
   Xen Orchestra REST API
```

## In practice

`xo vm list` — a table, with the container (host or pool) resolved to its name:

```console
$ xo vm list
ID                                    NAME    POWER STATE  MEMORY   CPUS  IP         CONTAINER
------------------------------------  ------  -----------  -------  ----  ---------  ---------
550e8400-e29b-41d4-a716-446655440001  web-01  Running      4.295GB  2     10.0.0.12  host-02
550e8400-e29b-41d4-a716-446655440002  db-01   Halted       4.295GB  4     -          prod-pool
550e8400-e29b-41d4-a716-446655440003  web-02  Running      1.074GB  1     10.0.0.13  prod-pool
```

`xo vm get <id>` — the same object as a detail view:

```console
$ xo vm get 550e8400-e29b-41d4-a716-446655440001
VM web-01  (Running)
IP:          10.0.0.12
Tags:        production, web
Container:   host-02
Template:    Oracle Linux 8
Memory:      4.295GB
CPUs:        2 (max 4)
Disks:       1  (xo vm vdis 550e8400-e29b-41d4-a716-446655440001)
Networks:    1
Boot:        hvm, order cda
Flags:       hvm auto-poweron
Created:     2026-01-02T10:00:00Z by admin
```

The same data, machine-readable — `--output json` (or the `--json` shortcut)
emits only the data, so it pipes cleanly into `jq`, and `--query`
([JMESPath](https://jmespath.org/)) projects and filters:

```console
$ xo vm list --output json | jq -r '.[].name_label'
web-01
db-01
web-02

$ xo vm list --query '[?power_state==`Running`].name_label'
web-01
web-02
```

Lifecycle actions are typed subcommands. They are asynchronous and return a
task id; add `--wait` (or set `XOA_WAIT=1`) to block until the task finishes:

```console
$ xo vm start 550e8400-e29b-41d4-a716-446655440002
Requested start of "db-01" (task task-123)
```

## Quick start

```sh
# 1. Install the binary (Linux and macOS)
curl -fsSL https://raw.githubusercontent.com/littlejo/xo-gocli/main/install.sh | sh

# 2. Store a profile (interactive, or via flags)
xo configure --profile lab \
  --endpoint https://xo.example.com \
  --token <token>

# 3. List VMs
xo vm list --profile lab

# 4. Machine-readable output
xo vm list --profile lab --output json | jq -r '.[].name_label'

# 5. Filter with a JMESPath query
xo vm list --profile lab --query '[?power_state==`Running`].name_label'
```

You can also select the profile with an environment variable:

```sh
export XOA_PROFILE=lab
xo vm list
```

A token (or a username + password) is required to authenticate. To get your
first token from a username and password, from the XO web UI or with
`xo-cli`, see [Getting a token](docs/getting-started/authentication.md#getting-a-token).
Windows users: download the `.zip` from the
[Releases page](https://github.com/littlejo/xo-gocli/releases) (see
[Install](docs/getting-started/install.md#windows)).

## Commands

`xo` is organized in resource groups. Each group exposes `list` and `get` plus
the domain operations Xen Orchestra exposes:

| Group | Operations |
| ----- | ---------- |
| `xo vm` | `list` `get` `create` `update` `delete` `start` `stop` `reboot` `pause` `unpause` `suspend` `resume` `snapshot` `export` `import` `vdis` `tag` |
| `xo host` | `list` `get` `tag` |
| `xo pool` | `list` `get` `rolling-update` `rolling-reboot` `emergency-shutdown` `tag` |
| `xo sr` | `list` `get` `scan` `reclaim-space` `tag` |
| `xo network` | `list` `get` `create` `create-internal` `create-bonded` `delete` `tag` |
| `xo vdi` | `list` `get` `create` `delete` `migrate` `export` `import` `tag` |
| `xo vbd` | `list` `get` `create` (attach a VDI to a VM) `delete` (detach) `connect` `disconnect` |
| `xo pbd` | `list` `get` `plug` `unplug` |
| `xo task` | `list` `get` `wait` `abort` |
| `xo token` | `list` `get` `create` |
| `xo template` | `list` `get` |
| `xo rest` | raw REST escape hatch: `xo rest <method> <path>` |

Plus `xo configure` (connection profiles), `xo version` and `xo completion`
(shell completion scripts). Every command has `--help`; all of them are in
the [command reference](docs/reference/cli/xo.md).

## Scope

`xo` wraps the Xen Orchestra **REST** API through the typed SDK v2 services —
the objects you manage day to day (`vm`, `host`, `pool`, `sr`, `network`,
`vdi`, `vbd`, `pbd`, plus `task`, `token` and `template`). It does **not**
wrap the legacy JSON-RPC/WebSocket surface: `xo-server`-only methods (users,
groups, servers, backup jobs, alarms, `xapi.*`, …) and the live event stream
(`watch`) are not first-class commands. The REST side of those is still
reachable through `xo rest`, and the typed command surface grows as the SDK
does. A side-by-side with the Node.js reference CLI — including where each is
broader — is in [docs/comparison.md](docs/comparison.md).

## Documentation

The [documentation index](docs/index.md) maps every page. The main ones:

| Page | Contents |
| ---- | -------- |
| [Install](docs/getting-started/install.md) | Install script, manual download, Windows, man pages, upgrade, uninstall |
| [Configuration](docs/getting-started/configuration.md) | Profiles, environment variables, global flags, debug, timeouts and waiting |
| [Guides](docs/guides/README.md) | Task-oriented walkthroughs (create a VM, snapshot and export, CI, …) |
| [Command guide](docs/reference/commands.md) | Per-group behavior: detail views, sync vs async, caveats |
| [Command reference](docs/reference/cli/xo.md) | Every command and flag, generated from `--help` (also shipped as man pages) |
| [Output and querying](docs/reference/output-and-query.md) | Formats, `--query`, the JSON/YAML contract |
| [Exit codes](docs/reference/exit-codes.md) | The `0` / `1` contract for scripts |
| [Troubleshooting](docs/troubleshooting.md) | Fixing a failing command |
| [Changelog](CHANGELOG.md) | What changed in each release |
| [Security policy](SECURITY.md) | Supported versions, how to report a vulnerability |

Contributors: start with [CONTRIBUTING.md](CONTRIBUTING.md) (build, tests,
release) and [AGENTS.md](AGENTS.md) (the rules every change must respect).

## License

This project is licensed under the [MIT License](https://opensource.org/licenses/MIT),
the same license as the Xen Orchestra Go SDK.
