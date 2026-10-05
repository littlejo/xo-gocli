# Comparison with `xo-cli`

[XO's reference CLI](https://github.com/vatesfr/xen-orchestra/blob/master/packages/xo-cli/README.md)
(`xo-cli`, a Node.js package) and this tool have different scopes: `xo-cli` is a
general-purpose, introspection-based client of `xo-server` (JSON-RPC over
WebSocket + a raw REST wrapper), described upstream as a *debug and power-user
tool*. This Go CLI is a typed, REST-only client with an AWS-CLI-like UX.

The comparison is split by audience: day-to-day interactive use first, then
automation and development, and finally the few remaining coverage gaps.

## For the user

How each tool feels when driven by hand:

| Aspect               | `xo-cli` (Node.js)                                                              | `xo` Go CLI (this project)                                                                             |
| -------------------- | ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| Command model        | Dynamic: `xo-cli <method> <param>=<value>` for **every** server method, discovered at runtime (`list-commands`) | Static, resource-oriented: `xo <resource> <operation>`, discovered via `--help`                        |
| Configuration        | Single registered instance (`xo-cli register` / `unregister`), token stored only, `--url` per-invocation override | Multiple named profiles (`xo configure --profile`), `$XOA_PROFILE` / `XOA_*` env vars, 0600 config file  |
| Authentication       | username/password, token, OTP (`--otp`), token validity control (`--expiresIn`) | username/password or token (via SDK v2)                                                                 |
| Object listing       | `list-objects` on all object types, with property filters; a single object needs `rest get vms/<id>` | Typed `list` per resource (`vm`, `host`, `pool`, `sr`, `network`, `vdi`, `vbd`, `task`, `template`) with `--limit` and resource filters (`--power-state`, `--type`, `--status`, `--vm`); single object via `xo <resource> get <id>` |
| Output formats       | Plain text or `--json`                                                          | `table` (default), `json`, `yaml`, `text`                                                               |
| Filtering / projection | `filter=` / `fields=` parameters (XO filter syntax)                           | AWS-CLI-like `--query` with JMESPath (incl. backtick literals)                                          |
| Task management      | `rest get tasks/<id> wait[=result]`, `rest post tasks/<id>/actions/abort`       | `task list` / `task get` (list filterable by `--status`); lifecycle actions return the task id          |
| VM lifecycle         | All methods (`vm.start`, `vm.stop`, `vm.reboot`, `vm.pause`, …)                 | `create`, `start` (host pinning), `stop` (clean/hard), `reboot` (clean/hard), `pause`/`unpause`, `suspend`/`resume`, `snapshot`, `delete` (confirm + `--yes`), `tag add/remove`, `update` (name/description/tags) |
| VM import / export (XVA) | `vm.import` / `vm.export` (file streaming with `@=`)                        | `vm export` (XVA/OVA to file or stdout, `--compress`) / `vm import` (XVA from file or stdin, `--pool`/`--sr`) |
| Pool maintenance       | `pool.rollingUpdate` / `pool.rollingReboot` / `pool.emergencyShutdown` methods | `pool rolling-update` / `rolling-reboot` / `emergency-shutdown` (confirm + `--yes` for the two destructive ones; synchronous) |
| Tagging              | Per-object methods (`vm.addTag`, `host.addTag`, …) via `rest` or the generic method call | `tag add` / `tag remove` on `vm`, `host`, `pool`, `sr` and `network` |
| Token management     | `create-token` (accepts same params as `register`)                              | `token list / get / create` with secret masking in output                                               |
| Live events          | `xo-cli watch [--ndjson]` (stream of notifications)                             | Not implemented                                                                                         |
| Raw REST escape hatch | `xo-cli rest get/post/patch/put/del` for any endpoint                          | `xo rest <method> <path>` (on top of SDK v2 HTTP facilities, not a second client)                       |
| Destructive operations | No confirmation prompt                                                         | Confirmation prompt + non-interactive `--yes`                                                           |
| TLS                  | `--allowUnauthorized` / `--au`                                                  | `--insecure` flag, per profile, or `$XOA_INSECURE`                                                       |

## For the developer

Automation, CI pipelines, and building on top of the CLI:

| Aspect            | `xo-cli` (Node.js)                                                    | `xo` Go CLI (this project)                                                              |
| ----------------- | --------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| Runtime           | Node.js (npm package — requires a Node.js installation)               | Static Go binary, no runtime dependencies                                                |
| Protocol          | JSON-RPC over WebSocket + REST wrapper                                 | REST API only, via `xenorchestra-go-sdk/v2`                                              |
| Parameters        | Untyped `param=value`, JSON values via `json:` prefix — mistakes surface at run time | Typed flags per command, validated at parse time                                        |
| Destructive in CI | No prompt: nothing blocks, but nothing protects either                | `--yes` for scripts; without it a non-terminal stdin is **rejected** rather than hanging, so a pipeline can never block or accidentally destroy |
| Machine-readable output | `--json` flag                                        | `--output json` / `yaml`; only the requested data goes to stdout, errors to stderr — `xo vm list --output json \| jq …` always works |
| Credentials in CI | Token stored by `register`                                            | `XOA_ENDPOINT` / `XOA_TOKEN` / … env vars override the profile per invocation — no config file to write or leak |
| Testing the CLI itself | Upstream tests only                                        | Unit tests (httptest fixtures) + opt-in integration tests against a live XO + a functional suite against the REST simulator, all in CI |

## Remaining coverage gaps

Everything above covers both tools; the rows below are the capabilities where
they diverge and that no other section states:

| Gap                                                                                     | `xo-cli` | `xo` Go CLI |
| --------------------------------------------------------------------------------------- | -------- | ----------- |
| Call *any* server method (servers, users, groups, backups, PBD, …)             | ✅        | ⬜ (added resource by resource on top of the SDK) |

Legend: ✅ available — ⬜ not available. A finer-grained, layer-by-layer view of
what is missing (and what already exists in the SDK) is in the
[roadmap](development.md#roadmap).

## Summary

- **Where `xo-cli` is broader**: it can call *every* `xo-server` method
  (hundreds: servers, users, groups, backups, PBD, network creation, …),
  stream live events, and reach any REST endpoint directly. For one-off or
  exotic operations it remains the most complete tool.
- **Where this CLI is better**: predictable and typed commands, multiple
  profiles, several output formats, JMESPath querying, non-interactive
  destructive operations, a static binary, and a REST-only architecture with
  no JSON-RPC/WebSocket dependency — better suited for automation, CI, and
  shell scripting.
- **What is still missing** (roadmap): `watch`, and more
  resources (servers, users, groups, backups, PBD). These are added as
  they are exposed by the Go SDK v2, per the
  [architecture rules](../AGENTS.md).
