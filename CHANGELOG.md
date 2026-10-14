# Changelog

All notable changes to `xo` are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
uses [Semantic Versioning](https://semver.org/) from 1.0.0 on: the command
names, flags, exit codes and the JSON/YAML output shape are the public API.

## [1.0.0] - Unreleased

First stable release. From here on, a breaking change to the commands, flags,
exit codes or machine output requires a new major version.

### Added

- Resource commands for the objects managed day to day, over the Xen
  Orchestra REST API through the official Go SDK v2:
  `vm`, `host`, `pool`, `sr`, `network`, `vdi`, `vbd`, `pbd`, `task`,
  `token` and `template`, each with `list` and `get` plus its domain
  operations (VM lifecycle, create, update, snapshot, export and import; pool
  rolling update, rolling reboot and emergency shutdown; VDI migrate, export
  and import; VBD attach, detach and hot-plug; tags).
- `xo rest`, a raw REST escape hatch for endpoints without a typed command.
- `xo configure`: AWS-style connection profiles (`--profile`,
  `$XOA_PROFILE`), with `list`, `show` and `remove`.
- Output formats `table` (default), `json`, `yaml` and `text`, the `--json`
  shortcut, and JMESPath filtering with `--query`.
- Names resolved in `list` and `get` tables (host, pool, SR, template) at a
  constant number of requests, whatever the number of rows.
- `--wait` and `$XOA_WAIT` on asynchronous actions, `xo task wait`, and
  `--wait-timeout` to bound a wait.
- `--yes` and `$XOA_YES` for destructive commands; without a terminal they
  fail instead of waiting for input.
- `--debug` and `$XOA_DEBUG` to show the raw SDK/API error behind a short one.
- Static binaries for Linux, macOS and Windows on amd64 and arm64, with
  checksums, and `install.sh` for Linux and macOS.

### Changed

- The wait deadline of `xo task wait` and of the `pool` maintenance commands
  is now `--wait-timeout`. `--timeout` means the per-request HTTP timeout on
  every command. A script that used `task wait --timeout 5m` must switch to
  `--wait-timeout 5m`: the old form still parses but no longer bounds the
  wait.
- `-v` no longer prints the version. Use `--version` or `xo version`.

### Fixed

- An unknown subcommand under a group (`xo vm strat <id>`) now fails with
  exit code 1 and a suggestion, instead of printing the help and exiting 0.
- `xo configure <typo>` is rejected instead of overwriting the selected
  profile with the `XOA_*` environment values.
- `vm create --template` accepts the template id printed by
  `xo template list`.

### Security

- `golang.org/x/text` updated to v0.43.0 (GO-2026-6629, GO-2026-5970; neither
  was reachable from `xo`).

## 0.x releases

Pre-1.0 versions (v0.1.1 to v0.35.x) are described in their
[GitHub release notes](https://github.com/littlejo/xo-gocli/releases).

[1.0.0]: https://github.com/littlejo/xo-gocli/releases/tag/v1.0.0
