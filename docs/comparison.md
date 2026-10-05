# Comparison with `xo-cli`

[XO's reference CLI](https://github.com/vatesfr/xen-orchestra/blob/master/packages/xo-cli/README.md)
(`xo-cli`, a Node.js package) and this tool have different scopes: `xo-cli` is
an introspection-based client of `xo-server` — every server method is callable
over JSON-RPC/WebSocket, plus a raw REST wrapper — while upstream itself
describes it as *"mainly a debug and power-user tool: there is no absolute
guarantee on its stability. Prefer the REST API for automation."* This Go CLI
is a typed, REST-only client with an AWS-CLI-like UX.

> **Basis of this comparison.** Both sides are verified against source, not
> marketing: `xo-cli` **v0.32.4** (`packages/xo-cli` of the
> `vatesfr/xen-orchestra` monorepo — `index.mjs`, `rest.mjs`, `config.mjs`,
> generated `README.md`) and this CLI at commit `cde603c` (2026-10-05). Where
> a row states a behavior, it was read in the code.

The comparison is split by audience: day-to-day interactive use first, then
automation and development, then the coverage gaps in both directions, and a
summary.

## For the user

How each tool feels when driven by hand:

| Aspect                 | `xo-cli` (Node.js)                                                                                        | `xo` Go CLI (this project)                                                                                                                                          |
| ---------------------- | --------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Command model          | Dynamic: `xo-cli <method> <param>=<value>` for **every** `xo-server` method (hundreds: `vm.*`, `pool.*`, `user.*`, `server.*`, `backup.*`, …) | Static, resource-oriented: `xo <resource> <operation>` — 13 resource groups, ~60 subcommands, each with a `Long` help and examples |
| Discovery              | `list-commands [--json] [<pattern>]…` — runtime introspection of the instance's methods and parameter types | `--help` tree at every level + shell completion (bash, zsh, fish, powershell)                                                                                        |
| Configuration          | Single registered instance (`register` / `unregister`); per-invocation override `--url token@host`; JSON config in `$XDG_CONFIG_HOME/xo-cli` written with the default umask (no explicit `0600`) | Multiple named profiles (`xo configure --profile`), `XOA_*` env overrides, YAML config written `0600`                                                                 |
| Authentication         | username/password, `--token`, OTP (`--otp`), token validity at register (`--expiresIn`, default one month); only the resulting token is stored | username/password or token per profile (via SDK v2); `token create --expires-in` to mint tokens; **no OTP**                                                         |
| Object listing         | `list-objects [--<prop>]… [<prop>=<value>]…` over **all** object types (always JSON output); single object via `rest get vms/<id>` | Typed `list` per resource (`vm`, `host`, `pool`, `sr`, `network`, `vdi`, `vbd`, `pbd`, `task`, `template`) with server-side XO filters (`--power-state`, `--type`, `--status`, `--vm`), `--limit`, and referenced names resolved in one batch per kind |
| Output formats         | Node `inspect` text, or `--json` (pretty) on `call` / `list-commands` / `rest get`; `list-objects` is JSON only | `table` (default, human-optimized), `json`, `yaml`, `text`                                                                                                          |
| Filtering / projection | `rest get` accepts `fields=`, `filter=` (XO filter syntax), `limit=`; `list-objects` `--<prop>` projection | AWS-CLI-like `--query` (JMESPath, incl. backtick literals) applied client-side on top of the same server-side XO filters                                            |
| Task management        | `rest get tasks/<id> [wait \| wait=result]`, `rest post tasks/<id>/actions/abort`                          | Typed `task list` (`--status` filter) / `get` / `wait` (`--timeout` wait deadline, exit code reflects success/failure/**interrupted**) / `abort` (confirmation)      |
| VM lifecycle           | All `vm.*` methods (`vm.start`, `vm.stop`, `vm.reboot`, `vm.pause`, …)                                     | 17 subcommands: `create`, `start` (host pinning, `--wait`), `stop` (clean/`--hard`), `reboot` (clean/`--hard`), `pause`/`unpause`, `suspend`/`resume`, `snapshot`, `delete` (confirm + `--yes`), `tag add/remove`, `update` (name/description/tags), `vdis`, `export`, `import` |
| VM export / import     | `vm.export vm=<id> @=vm.xva` (streaming, `format=xva\|ova` supported server-side); `vm.import` is **XVA only** (OVA needs the separate `xo-upload-ova` tool) | `vm export` (XVA or OVA, to file or stdout, `--compress`) / `vm import` (XVA from file or stdin, `--pool`/`--sr`) — plus `vdi export`/`import` (raw/VHD) and `vdi migrate` |
| Pool maintenance       | `pool.rollingUpdate` / `pool.rollingReboot` / `pool.emergencyShutdown` return a task id — you wait separately with `rest get tasks/<id> wait=result` | `pool rolling-update` / `rolling-reboot` / `emergency-shutdown`: synchronous (the command blocks on the task), confirmation + `--yes` on the destructive ones, optional `--timeout` wait deadline |
| Tagging                | Per-object methods (`vm.addTag`, `host.addTag`, …) via the generic call                                    | `tag add` / `tag remove` on `vm`, `host`, `pool`, `sr`, `network` and `vdi`                                                                                          |
| Token management       | `create-token` (same parameters as `register`); the token is printed                                      | `token list` / `get` / `create` (`--description`, `--expires-in`, `--client-id`) with the secret **masked** in `list` output (table and JSON)                        |
| Live events            | `xo-cli watch [--ndjson]` — WebSocket notification stream, per-method counts on Ctrl+C                    | Not implemented (documented gap, see [roadmap](development.md#roadmap))                                                                                             |
| Raw REST escape hatch  | `rest get/post/put/patch/del`; `get` can save the response to a file (`--output <file\|dir>`) and `wait` on tasks; `post/put/patch` stream a body with `--input` | `xo rest <method> <path>` with `--param`, `--header`, `--data` (JSON, `-` = stdin), `--include`, `--query` on the result; sent through the SDK v2 HTTP client (same auth/TLS — not a second client) |
| Transfer feedback      | Progress on stderr during import/export/rest transfers: `% of total @ speed/s — ETA` (`progress-stream`)  | Silent transfer, bounded by the global `--timeout` (30 s default, documented to raise it for large archives)                                                       |
| Destructive operations | No confirmation prompt                                                                                    | Confirmation prompt; `--yes` / `$XOA_YES` for automation; a non-terminal stdin is **rejected** rather than hanging                                                  |
| TLS                    | `--allowUnauthorized` / `--au` (global flag)                                                              | `--insecure` per profile (`xo configure -k`) or `$XOA_INSECURE`                                                                                                      |

## For the developer

Automation, CI pipelines, and building on top of the CLI:

| Aspect                  | `xo-cli` (Node.js)                                                                                      | `xo` Go CLI (this project)                                                                                                                                            |
| ----------------------- | ------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Runtime                 | Node.js `>= 15.3` + an npm dependency tree (xo-lib, undici, …)                                          | Static Go binary, no runtime dependencies                                                                                                                              |
| Protocol                | JSON-RPC over WebSocket for every method, plus raw REST (`rest`) for the REST-only surface              | REST API only, via `xenorchestra-go-sdk/v2` (cookie auth, single client)                                                                                               |
| Parameters              | Untyped `param=value`; `json:` prefix for JSON values, `true`/`false` booleans, everything else a string — mistakes surface at run time (server schema errors are pretty-printed) | Typed flags per command, validated at parse time (`invalid --template id "…" (expected a UUID)`, `--limit must be >= 0`, …)                                          |
| Exit codes              | `0` on success — **except**: a method that returns a number exits with that number (`server.add` → exit `42`); errors → `1` with `✖ …` on stderr | Stable contract pinned by process-level tests: `0` success, `1` error; `Error: …` on **stderr**, nothing on stdout — safe in pipelines and `set -e`                 |
| Machine-readable output | `--json` (pretty-printed); `list-objects` is JSON-only                                                  | `--output json` / `yaml` (+ `--query` projection); only the requested data goes to stdout, errors to stderr — `xo vm list --output json \| jq '.[].name_label'` always works |
| Credentials in CI       | Stored token from `register`, or `--url token@host` (the token sits on the command line / shell history) | `XOA_ENDPOINT` / `XOA_TOKEN` / … env vars override the profile per invocation — no token on argv, no config file to write or leak                                    |
| Timeouts                | None — the HTTP agent is created with `headersTimeout: 0, bodyTimeout: 0`                               | Per-request timeout (`--timeout` / `$XOA_TIMEOUT`, 30 s default) plus wait deadlines (`--timeout` on `task wait` and pool maintenance) — a stuck task cannot hang a poll |
| Destructive in CI       | No prompt: nothing blocks, but nothing protects either                                                  | `--yes` for scripts; without it a non-terminal stdin is **rejected** rather than hanging, so a pipeline can never block or accidentally destroy                       |
| Testing the CLI itself  | Upstream tests only                                                                                     | Unit tests (httptest fakes, anti-N+1 request counts, secret masking) + opt-in integration tests against a live XO + a functional suite against the REST simulator in CI, with `-race` |
| License                 | AGPL-3.0-or-later (part of the `xen-orchestra` monorepo)                                                 | MIT                                                                                                                            |

## Remaining coverage gaps

The capabilities where the two tools diverge, in **both** directions:

| Gap                                                                       | `xo-cli` | `xo` Go CLI |
| ------------------------------------------------------------------------- | -------- | ----------- |
| Call *any* `xo-server` method (users, groups, servers, backup jobs, alarms, `xapi.*`, …) | ✅ | ⬜ (REST side reachable via `xo rest`; JSON-RPC-only methods are not — resources are added on top of the SDK as it grows) |
| Live event stream (`watch` / notifications)                                | ✅       | ⬜ |
| Runtime introspection of instance methods (`list-commands`)                | ✅       | ⬜ (by design: static commands) |
| Cross-type object listing (`list-objects`)                                 | ✅       | ⬜ (per-type typed lists; `xo rest get <collection>` covers the raw REST side) |
| OTP at registration                                                        | ✅       | ⬜ |
| Save a REST response to a file (`rest get --output`) and `wait` on a task from `rest get` | ✅ | ⬜ (downloads are covered by the typed `vm export` / `vdi export` instead) |
| Progress display during transfers (`% @ speed — ETA`)                      | ✅       | ⬜ (bounded silent transfer; documented) |
| OVA import                                                                 | ⬜ (separate `xo-upload-ova` tool) | ⬜ (XVA only, same as `xo-cli`) |
| Shell completion                                                           | ⬜       | ✅ (bash, zsh, fish, powershell) |
| Multiple named profiles / env-var profiles                                 | ⬜       | ✅ |
| JMESPath `--query` projection                                              | ⬜       | ✅ |
| YAML / text output formats                                                 | ⬜       | ✅ |
| Config file written with explicit `0600`                                   | ⬜ (default umask) | ✅ |
| Request timeouts                                                           | ⬜       | ✅ |
| Confirmation prompts for destructive operations                            | ⬜       | ✅ |

Legend: ✅ available — ⬜ not available. A finer-grained, layer-by-layer view of
what is missing (and what already exists in the SDK) is in the
[roadmap](development.md#roadmap).

## Summary

- **Where `xo-cli` is broader**: it can call *every* `xo-server` method
  (hundreds — servers, users, groups, backups, alarms, …), stream live events
  (`watch`), introspect the instance's own methods (`list-commands`), list
  objects across all types (`list-objects`), download REST responses to files,
  and show transfer progress. For one-off or exotic operations it remains the
  most complete tool — and it is the only client for JSON-RPC-only methods.
- **Where this CLI is better**: a stable, typed and documented command surface
  with help and shell completion; multiple profiles and env-based
  configuration; four output formats plus JMESPath querying; a pinned
  `0`/`1` exit-code contract with errors on stderr; confirmation +
  non-interactive `--yes` for destructive operations; explicit `0600`
  credentials; real timeouts (request and wait deadlines) where `xo-cli` has
  none; and a static binary on a REST-only architecture with no
  JSON-RPC/WebSocket dependency — better suited for automation, CI, and shell
  scripting.
- **Where the two are equivalent**: VM lifecycle and XVA export/import,
  pool maintenance (the Go CLI waits synchronously; `xo-cli` returns a task
  id and waits separately), token creation, and a raw REST escape hatch.
- **What is still missing** (roadmap): `watch`/events, and the JSON-RPC-only
  resources (users, groups, servers, backup jobs). These are added as they are
  exposed by the Go SDK v2, per the [architecture rules](../AGENTS.md).
