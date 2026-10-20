# SDK v2: what we build on

This is the result of inspecting `xenorchestra-go-sdk` (pinned in `go.mod`)
before implementing each command, per the SDK-first rule in
[AGENTS.md](../../AGENTS.md). Refresh this page whenever the SDK is upgraded
(module layout, entry points, request model and known gaps).

- [Module layout](#module-layout)
- [Two entry points](#two-entry-points)
- [Authentication & transport](#authentication--transport)
- [Request model (as the SDK sends it)](#request-model-as-the-sdk-sends-it)
- [Known SDK gaps the CLI works around](#known-sdk-gaps-the-cli-works-around)
- [Which command uses which SDK surface](#which-command-uses-which-sdk-surface)

SDK bugs found along the way are tracked in the
[roadmap](roadmap.md#sdk-bug-to-verify-upstream).

## Module layout

The SDK is a **single Go module** (`github.com/vatesfr/xenorchestra-go-sdk`,
pinned at `v1.21.0` in `go.mod`) that contains **both** APIs:

| Path | API | Used by xo-gocli |
| ---- | --- | ---------------- |
| `client/`, `pkg/services/jsonrpc/` | v1: JSON-RPC over WebSocket | **never** |
| `v2/` | v2: REST client + typed services | **yes — the only API layer** |
| `pkg/config/` | shared config type | yes (via `v2`) |
| `pkg/payloads/` | shared REST response types | yes (returned by the typed services) |

There is no `v2/go.mod`: `v2` is a plain subdirectory, so the import path
`github.com/vatesfr/xenorchestra-go-sdk/v2` is a subpackage of the v1 module
— the `/v2` suffix is a directory name, not a Go major-version path. The
v1.21.0 version number is the *module* version, not the REST API version
(the REST API itself is `/rest/v0`).

> **SDK pin history.** The module was briefly `require`d at `v1.19.0` with a
> `replace` to a fork commit carrying the `VBD.Position` fix (upstream PR
> #119, not yet released then): `payloads.VBD.Position` was typed
> `StringifiedInt`, but the REST API returns the XAPI `userdevice` under the
> `position` key as a string whose content is data dependent (`"0"` or a
> device name such as `"xvdb"`), so every `VBD().Get` / `GetAll` failed to
> unmarshal. PR #119 shipped in `v1.20.0`, so the `replace` was removed and
> the `require` bumped to the plain upstream release (no fork, no
> pseudo-version).

## Two entry points

1. **`v2.New(cfg) → library.Library`** — the typed facade. Returns
   `VM()`, `Host()`, `Pool()`, `SR()`, `Network()`, `Task()`, `VDI()`, `VBD()`,
   `PBD()` and, since `v1.21.0`, `Template()`. Every service method is a REST call through the internal
   `v2/client` (`client.TypedGet` & co.), returning `pkg/payloads` structs.
   This is what `internal/cli.NewClient` builds and what most commands use.
2. **`v2/client.New(cfg) → *client.Client`** — the raw REST client. The SDK
   *exports* `HttpClient`, `BaseURL` and `AuthToken` precisely so callers can
   reach endpoints the SDK does not (yet) wrap. `internal/cli.NewHTTPClient`
   is built on it and powers `xo rest`, `xo token`, `xo template`, `xo task`,
   the VM PATCH in `xo vm update`, `xo vm export` / `import`, the `--wait`
   poll loop (`internal/taskwait`). That is the documented escape hatch, not a second REST
   client.

   Most of these calls go through the SDK's `client.TypedGet` & co. Four
   files build a raw `http.Request` themselves and send it with the SDK
   client's `HttpClient`, because the SDK has no helper for that request
   shape: `internal/commands/rest/rest.go` (any method, headers, raw body),
   `internal/commands/vm/xva.go` (XVA/OVA streaming),
   `internal/commands/vm/update.go` (the PATCH) and
   `internal/commands/token/token.go` (token endpoints, POST replayed over a
   307). They still use the SDK's base URL, authentication cookie and TLS
   settings.

## Authentication & transport

- Token mode: every request carries the token as an
  `authenticationToken` **cookie** (set by `doRequest`, the SDK's own
  mechanism). The CLI never sends `Authorization` headers itself.
- Username/password mode: `client.New` logs in via
  `POST /auth/login` (base URL **without** `/rest/v0`) and keeps the token
  cookie from the response.
- `ws`/`wss` endpoints are transparently rewritten to `http(s)`;
  `InsecureSkipVerify` is applied on a cloned transport; the HTTP client
  timeout is 30 s by default (`cfg.ClientTimeout`), overridable per
  invocation with the global `--timeout` flag or `$XOA_TIMEOUT` (wired to
  `ClientTimeout` in `internal/cli.buildSDKConfig`).
- `v2/xo.go` also holds a **lazy** v1 client (`V1Client()`), created only if
  JSON-RPC is actually requested. The CLI never calls it, so no WebSocket is
  ever opened.

## Request model (as the SDK sends it)

- Base URL is `<endpoint>/rest/v0`; endpoints beginning with `api/` are sent
  without that prefix.
- Collection reads: `GET /<resource>` with `fields`, `limit`, `filter`
  (the XO live-filter syntax) query parameters.
- Single object: `GET /<resource>/<id>`.
- Actions: `POST /<resource>/<id>/actions/<name>` → `{"taskId": …}` (202,
  asynchronous); e.g. `start`, `clean_shutdown`, `hard_shutdown`,
  `clean_reboot`, `hard_reboot`, `pause`, `unpause`, `suspend`, `resume`,
  `snapshot`.
- Resource create (synchronous): `POST /<resource>` with a JSON body →
  `{"id": …}` (the new object's id, no task); used by `vdi create`
  (`POST /vdis`) and `vbd create` (`POST /vbds`).
- Delete: `DELETE /<resource>/<id>` → `{"success": true}` (synchronous, no
  task id); e.g. `DELETE /vms/<id>`.
- Tags: `PUT`/`DELETE /<resource>/<id>/tags/<tag>` (`vms`, `hosts`,
  `pools`, `srs`, `networks`), via the typed `AddTag` / `RemoveTag` of the
  `Taggable` services.
- Partial update: `PATCH /<resource>/<id>` with **camelCase** JSON fields
  (`nameLabel`, `nameDescription`, …) — responses, in contrast, use
  snake_case.
- Non-2xx responses surface as `API error: <status> - <body>`; the CLI maps
  a 404 (checked on the actual HTTP status carried by the error, not a
  substring) to a concise "not found" message, and translates the well-known
  SDK/Go transport and parsing errors — timeout, unreachable endpoint,
  malformed API response, 401/403 — into concise messages (see
  `internal/cli/errors.go`). In every case the raw error is kept behind
  `--debug` (`$XOA_DEBUG`).
- `pkg/config.NewWithValues` (what we use) takes explicit values and reads no
  environment variables; `pkg/config.New` (env `XOA_*`) is not used.

## Known SDK gaps the CLI works around

| Gap | CLI workaround |
| --- | -------------- |
| `VM().Update` returns `not yet implemented` | `xo vm update` sends the PATCH itself via the exported `*client.Client` (documented in `vm/update.go`; to contribute upstream) |
| No typed VM XVA/OVA import/export | `xo vm export` / `xo vm import` stream through the exported `*client.Client` against `GET /vms/<id>.<format>` and `POST /pools/<pool>/vms` (documented in `vm/xva.go`; to contribute upstream) |
| No typed service for tasks (beyond `Task()`, see below) or user tokens | `TypedGet` directly (`xo task`, `xo token`) |
| The typed `Template()` service (since `v1.21.0`) returns `payloads.Template`, which keeps only the fields it models | the name resolver uses it; `xo template list` / `get` still read the raw `vm-templates` objects with `TypedGet`, so their JSON keeps every field XO returns |
| `Task().Wait` / `WaitWithTimeout` unmarshal into `payloads.Task`, whose `Result` is a struct — but XO sometimes returns a task `result` as a **plain string** (see `TestTaskGetStringResult`), so `Get` (and therefore `Wait`) fails to unmarshal such tasks and loops to the deadline. (`Wait` treats `interrupted` as terminal since `v1.21.0`, [vatesfr/xenorchestra-go-sdk#122](https://github.com/vatesfr/xenorchestra-go-sdk/pull/122), which closed [#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121).) | `xo task wait` and `--wait` poll with `client.TypedGet` (the same single SDK boundary as `task get`) and stop on `success`/`failure`/`interrupted` (documented in `internal/taskwait`; the SDK result type should be contributed upstream). The pool-maintenance commands go through the SDK's synchronous wait: an interrupted task now fails them at once, and the SDK's empty `<action> failed: ` message is completed by the CLI's error translation; `--wait-timeout` and Ctrl+C bound a task that stays pending |
| `users/me` 307-redirects to the user id | handled by `net/http` following the redirect; `doTokensRequest` relies on 307 body replay for POST |
| `v2` package `init()` runs `gotenv.Load()` (reads a `.env` in the CWD) | harmless: we build the config with `NewWithValues`, which reads no env vars |
| Server XO version is not exposed by the REST API (v0) — `GET /ping` only returns `{result, timestamp}` and the `xoa` REST controller has no version route | `xo version` prints the **CLI** version offline (like `aws version`); the server version exists only as the legacy JSON-RPC `getServerVersion` method, which the CLI never calls (v1 is forbidden by [AGENTS.md](../../AGENTS.md)) |

## Which command uses which SDK surface

| Command | SDK surface |
| ------- | ----------- |
| `vm list/get/create/start/stop/reboot/pause/unpause/suspend/resume/snapshot/delete/tag` | typed `library.VM` |
| `vm vdis` | typed `library.VM` (`GetVDIs`) |
| `vdi list/get/create/delete` | typed `library.VDI` (`GetAll`/`Get`/`Create`/`Delete`, all synchronous) |
| `vdi migrate` | typed `library.VDI` (`Migrate`, returns a task id — the VDI gets a new id) |
| `vdi tag add/remove` | typed `library.VDI` (`AddTag` / `RemoveTag`, `Taggable`) |
| `vdi export/import` | typed `library.VDI` (`Export` / `Import`, raw/vhd streaming) |
| `vbd list/get/create/delete` | typed `library.VBD` (`GetAll`/`Get`/`Create`/`Delete`, all synchronous) |
| `vbd connect/disconnect` | typed `library.VBD` (`Connect`/`Disconnect`, return a task id) |
| `pbd list/get` | typed `library.PBD` (`GetAll`/`Get`, synchronous) |
| `pbd plug/unplug` | typed `library.PBD` (`Plug`/`Unplug`, return a task id) |
| `vm export/import` | raw `*client.Client` (XVA/OVA streaming — SDK gap) |
| `host/pool/sr/network list/get/tag` | typed `library.{Host,Pool,SR,Network}` (`AddTag` / `RemoveTag`) |
| `pool rolling-update / rolling-reboot / emergency-shutdown` | typed `library.Pool` (`RollingUpdate` / `RollingReboot` / `EmergencyShutdown`, synchronous: they wait for the backing task) |
| `network create / create-internal / create-bonded` | typed `library.Network` (`Create*`, which delegate to the `Pool` create actions and wait for the task) |
| `network delete` | typed `library.Network` (`Delete`, synchronous) |
| `sr scan/reclaim-space` | typed `library.SR` (`Scan` / `ReclaimSpace`) |
| `task wait` | raw `client.TypedGet` poll loop (the SDK's `Task().Wait` is not used — see "Known SDK gaps") |
| `task abort` | typed `library.Task` (`Abort`) + raw `client.TypedGet` pre-check (existence and status, same reason as `task wait`) |
| `vm update` | raw `*client.Client` (PATCH — SDK gap) |
| `template list/get`, `task list/get` | raw `client.TypedGet` |
| `token list/get/create` | raw `*client.Client` (GET/POST, 307 redirect) |
| `rest` | raw `*client.Client` (full method set) |
| `version` | none (offline — prints the CLI version only) |
