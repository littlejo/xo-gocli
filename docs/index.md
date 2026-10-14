# `xo` documentation

Start from what you want to do. The [README](../README.md) has the overview
and a five-minute quick start.

## Using `xo`

| Page | Read it to… |
| ---- | ----------- |
| [Install](getting-started/install.md) | install, upgrade or uninstall `xo` (script, manual download, Windows, man pages, from source) |
| [Authentication](getting-started/authentication.md) | get a token, and choose between a token and a password |
| [Configuration](getting-started/configuration.md) | set up profiles, environment variables, global flags, insecure and debug modes, timeouts and waiting |
| [Shell completion](getting-started/shell-completion.md) | enable completion in bash, zsh, fish or PowerShell |
| [Guides](guides/README.md) | follow a task end to end: create a VM from a template, snapshot and export, several pools, CI, rolling updates, bulk actions |
| [Use cases](usecases.md) | add a disk to a VM, step by step |
| [Troubleshooting](troubleshooting.md) | fix a failing command |

## Reference

| Page | Contents |
| ---- | -------- |
| [Command reference](reference/cli/xo.md) | every command and flag, generated from `--help` (also shipped as man pages) |
| [Command guide](reference/commands.md) | what `--help` does not say: detail views, sync vs async actions, caveats, one section per command group |
| [Output and querying](reference/output-and-query.md) | output formats, `--query`, the JSON/YAML contract, mutation output, human view conventions |
| [Exit codes](reference/exit-codes.md) | the `0` / `1` contract for scripts |
| [Comparison with `xo-cli`](comparison.md) | side-by-side with the Node.js reference CLI |
| [Changelog](../CHANGELOG.md) | what changed in each release |
| [Security policy](../SECURITY.md) | supported versions, how to report a vulnerability |

## Contributing

| Page | Contents |
| ---- | -------- |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | toolchain, build, tests, commits and versioning, pull requests, CI and release |
| [AGENTS.md](../AGENTS.md) | the hard rules every change must respect (humans and AI agents) |
| [Architecture](internals/architecture.md) | package layout, the `list` vs `get` convention, invariants |
| [SDK v2](internals/sdk-v2.md) | what the SDK provides, its gaps, and which command uses which SDK surface |
| [Performance](internals/performance.md) | the anti-N+1 cost model, how to measure it, reference numbers |
| [Roadmap](internals/roadmap.md) | what is not built yet, and known SDK bugs |
| [Road to v1.0.0](archive/road-to-v1.md) | the historical pre-1.0 audit (archive) |
