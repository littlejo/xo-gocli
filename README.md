# xo Go CLI

A modern command line client for [Xen Orchestra](https://github.com/vates/xen-orchestra),
written in Go with an AWS-CLI-like experience.

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

## Quick start

```sh
# 1. Install the binary
curl -fsSL https://raw.githubusercontent.com/littlejo/xo-gocli/main/install.sh | sh

# 2. Store a profile (interactive, or via flags)
xo configure --profile lab \
  --endpoint https://xo.example.com \
  --token <token>

# 3. List VMs
xo vm list --profile lab

# 4. Get machine-readable output
xo vm list --profile lab --output json | jq '.[].name_label'

# 5. Filter with a JMESPath query
xo vm list --profile lab --query '[?power_state==`Running`].name_label'
```

You can also select the profile with an environment variable:

```sh
export XOA_PROFILE=lab
xo vm list
```

## Features

- Resource-oriented commands (`xo vm list`, `xo host list`, `xo template list`, `xo sr list`, `xo pool list`, …)
- Multiple connection profiles, AWS-style (`--profile`, `$XOA_PROFILE`)
- Human-friendly default output plus `--output json|yaml|text` (or the `--json` shortcut, or `$XOA_DEFAULT_OUTPUT`) for scripting
- AWS-CLI-like `--query` using [JMESPath](https://jmespath.org/)
- Concise errors by default; `-d`/`--debug` (or `$XOA_DEBUG`) reveals the raw SDK/API error on failure
- Self-signed / internal certificate escape hatch: `xo configure --insecure` (stored per profile) or `$XOA_INSECURE`
- Static, dependency-free binaries for Linux, macOS and Windows — amd64 and arm64 only (the SDK does not compile on 32-bit). On Windows the binary is a `.zip` on the releases page; `install.sh` covers Linux and macOS

## Commands

`xo` is organized in resource groups. Each group exposes `list` and `get`
plus the domain operations Xen Orchestra exposes (`xo rest` is a raw REST
escape hatch, not a resource):

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
(shell completion scripts).

Every flag and subcommand is documented in [docs/usage.md](docs/usage.md).

## Documentation

| Page | Contents |
| ---- | -------- |
| [Usage](docs/usage.md) | Installation, configuration, every command, output & querying |
| [Use cases](docs/usecases.md) | Task-oriented recipes (add a disk to a VM, …) |
| [Development](docs/development.md) | Architecture, toolchain, testing, CI & release, versioning, roadmap, license |
| [Comparison with `xo-cli`](docs/comparison.md) | Side-by-side with the reference XO CLI |

Architecture rules for contributors live in [AGENTS.md](AGENTS.md).

## License

This project is licensed under the [MIT License](https://opensource.org/licenses/MIT),
the same license as the Xen Orchestra Go SDK.
