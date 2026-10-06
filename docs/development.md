# Development

How to build, test, and release `xo`. The repository is set up for AI-assisted
and local development — see [AGENTS.md](../AGENTS.md) for the architecture
rules and conventions (the important one: `xenorchestra-go-sdk/v2` is the only
Xen Orchestra API layer).

## Table of contents

- [Architecture](#architecture)
- [Toolchain](#toolchain)
- [Testing](#testing)
  - [Functional tests against the simulator](#functional-tests-against-the-simulator)
  - [Process-level tests (the scripting contract)](#process-level-tests-the-scripting-contract)
- [CI / Release](#ci--release)
- [Versioning](#versioning)
- [Repository layout](#repository-layout)
- [Performance](#performance)
- [SDK v2: what we build on](#sdk-v2-what-we-build-on)
  - [Module layout](#module-layout)
  - [Two entry points](#two-entry-points)
  - [Authentication & transport](#authentication--transport)
  - [Request model (as the SDK sends it)](#request-model-as-the-sdk-sends-it)
  - [Known SDK gaps the CLI works around](#known-sdk-gaps-the-cli-works-around)
  - [Which command uses which SDK surface](#which-command-uses-which-sdk-surface)
- [Roadmap](#roadmap)
  - [Done](#done)
  - [Layer 1 — already in the SDK, not exposed by the CLI yet](#layer-1--already-in-the-sdk-not-exposed-by-the-cli-yet)
  - [Layer 2 — complete SDK services, no CLI resource yet](#layer-2--complete-sdk-services-no-cli-resource-yet)
  - [Layer 3 — in the REST API, not in the SDK yet](#layer-3--in-the-rest-api-not-in-the-sdk-yet)
  - [Layer 4 — not in the SDK at all](#layer-4--not-in-the-sdk-at-all)
  - [CLI-side polish (no SDK dependency)](#cli-side-polish-no-sdk-dependency)
  - [SDK bug to verify upstream](#sdk-bug-to-verify-upstream)
- [License](#license)

## Architecture

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

Commands stay thin: they resolve flags, call the SDK, and hand the result to
the output layer. SDK gaps (for example the missing typed VM update) are
reached through the SDK's own HTTP facilities, documented in the code, and
contributed upstream where appropriate. The full rules are in
[AGENTS.md](../AGENTS.md).

## Toolchain

A [mise](https://mise.jdx.dev/) config pins the Go toolchain and
golangci-lint (`mise.toml`):

```sh
mise install          # install the pinned Go toolchain and golangci-lint
mise run build        # go build -o dist/xo ./cmd/xo
mise run test         # go test ./...
mise run lint         # golangci-lint run + go vet + gofmt check
```

Without mise, a Go 1.26+ toolchain and golangci-lint (v2.14.0) work
directly:

```sh
go build -o dist/xo ./cmd/xo
go test ./...
golangci-lint run && go vet ./... && test -z "$(gofmt -l .)"
```

`golangci-lint` (v2.14.0) is the reference linter; the version pinned in
`mise.toml` matches the one CI runs on every push.

## Testing

Unit tests run without a Xen Orchestra instance (they use `httptest` servers
and deterministic fixtures). Integration tests are opt-in and only run when
explicitly enabled:

```sh
export XOA_TEST_URL=https://xo.example.com
export XOA_TEST_TOKEN=<token>
go test -tags=integration ./...
```

Without those variables the integration tests are reported as **skipped**,
never as passed. Integration credentials must never be committed; they are
configured through the environment.

### Functional tests against the simulator

Functional tests run the same `-tags=integration` suite against the
[xo-api-sim](https://github.com/vatesfr/xo-api-sim) REST API simulator, so the
CLI is exercised end-to-end with real HTTP requests but without a live Xen
Orchestra instance or any credential. In CI this is the `functional` job;
locally you can point the suite at a simulator you run yourself:

```sh
# terminal 1: start the simulator (see the xo-api-sim repo)
npm ci && npm run build
PORT=3001 AUTH_TOKEN=test-token node dist/index.js

# terminal 2: run the functional suite against it
XOA_TEST_URL=http://localhost:3001 XOA_TEST_TOKEN=test-token go test -tags=integration ./...
```

> Note: the SDK v2 client authenticates with an `authenticationToken` cookie,
> while xo-api-sim (as released) only reads the `Authorization: Bearer` header.
> `ci/xo-api-sim-cookie-auth.patch` closes that gap and is applied in CI; run
> the simulator from that patched source (or the fix contributed upstream)
> locally.

### Process-level tests (the scripting contract)

`cmd/xo/process_test.go` runs the **compiled** binary as a child process
(built once per test run with `go build`, then driven with `os/exec`) against
an in-process `httptest` fake of the XO REST API. These tests pin the
scripting contract that in-process unit tests cannot see — exit codes, the
stdout/stderr split, the `Error:` prefix, `--help` on every command group,
401 handling, SIGINT (Ctrl+C) propagation all the way down to the HTTP layer,
profile resolution from the config file, and the exact JSON shape. This is
the class of test that catches regressions like the `xo rest` flag-merge
panic, which slipped through a fully green in-process suite. They need no
live instance and run in the normal unit suite.

The JSON shape is pinned by golden files in `cmd/xo/testdata/`. After an
**intentional** output change, regenerate them and review the diff:

```sh
go test ./cmd/xo/ -update
git diff cmd/xo/testdata/
```

## CI / Release

- **CI** (`.github/workflows/ci.yml`): runs on push to `main` and on every PR —
  `gofmt`, `go vet`, `golangci-lint`, unit + integration tests (including a
  `-race` pass to catch data races early), build. A second `functional` job
  spins up the xo-api-sim REST simulator (pinned commit, cookie-auth patch) and
  runs the integration suite against it, so every push is tested end-to-end
  over real HTTP without a live instance. The workflow is also reusable
  (`workflow_call` with a `ref` input) so other workflows — in particular the
  release — can run the exact same gates on a specific ref.
- **Version** (`.github/workflows/version.yml`): on every push to `main`,
  computes the next semver tag from the conventional-commits history
  (`feat` → minor, anything else → patch, **documentation-only changes → no
  new tag**), pushes it, and triggers the Release workflow.
- **Release** (`.github/workflows/release.yml`): for a `v*.*.*` tag, runs the
  full CI gate suite (reusing the CI workflow against the resolved tag ref, so
  a release is only built on a commit that passed exactly the same gates as
  CI), then builds cross-platform binaries with
  [GoReleaser](https://goreleaser.com) and publishes them as a draft GitHub
  release. It can also be run manually from the Actions tab (optionally
  targeting a specific tag).

Normal flow: push to `main` — the tag and the release are created
automatically. To release a specific commit by hand:

```sh
git tag v1.0.0 && git push origin v1.0.0
```

## Versioning

Releases follow [semantic versioning](https://semver.org/) and
[Conventional Commits](https://www.conventionalcommits.org/): a `feat` commit
bumps the minor version, any other change bumps the patch version. Tags are
created automatically on push to `main` (see
[CI / Release](#ci--release)).

A change that only touches documentation (`docs/`, `README.md`, `AGENTS.md`)
does not produce a new tag: a release ships the binary and `go.mod`, nothing
in the documentation, so doc fixes are not released on their own. The rule is
implemented in `.github/scripts/bump-version.sh`.

## Repository layout

```text
cmd/xo/                  main entrypoint
internal/
  cli/                   SDK client construction, global flags, error hints,
                           name-resolver factory (NewResolver)
  commands/              one package per resource group (vm, host, pool, …)
    configure/           profile management
  config/                profiles, environment overrides, config file
  output/                table/json/yaml/text rendering, JMESPath queries,
                           detail-sheet helpers (DetailField, OrDash, JoinNames)
  resolve/               reference→name resolution (single + batch), the
                           shared anti-N+1 helper behind every `get` detail view
  taskwait/              shared --wait poll loop (reused by task wait)
```

### `list` vs `get` (the detail-view convention)

`list` is a **selection view**: a flat, one-line-per-resource table used to
pick a resource. `get` is a **detailed inspection view**: a key/value **detail
sheet** (see `output/detail.go`) that resolves the resource's relationships to
**names** instead of UUIDs, so the sheet is complete on its own.

The `get` detail sheets all follow the same rules:

- **Cost model (anti-N+1).** A single reference (a container, a master, the SR
  of a VDI, the VM/VDI of a VBD, the host/SR/pool of a PBD, the pool of a
  network/host/template) costs one GET. A *collection* (the hosts of a pool,
  the PBDs of an SR, the VBDs of a VDI, the resident VMs of a host) is resolved
  with **one batch call + an id→name map** (`resolve.*BatchNames` /
  `HostsOfPool`), never a GET per element. The cost is therefore constant,
  independent of pool/farm size, and the tests assert it against an
  `httptest` server that counts requests.
- **Never fails on resolution.** A reference that cannot be resolved falls back
  to its raw id (or the line is omitted), so the view is always complete and
  the command still succeeds.
- **Machine output is untouched.** `--output json`/`yaml`/`text` and `--query`
  always emit the raw object / projection, exactly as before; only the default
  human `table` view is the detail sheet.

`list` tables follow the same rules for their **reference columns**: a column
that points at another object (the `MASTER` of a pool, the `SR` of a VDI, the
`VM`/`VDI` of a VBD, the `POOL` of a host, the `CONTAINER` of a
VM/SR) shows the **name** of the referenced object instead of its UUID, by
default. The names come from **one batch call per referenced kind**
(`resolve.*BatchNames`), never a lookup per row, so the cost is constant
whatever the list size. A mixed reference (a container that is a pool or a
host) is disambiguated with the sibling field (`$poolId` for VMs, `$pool` for
SRs). Two extra rules apply to lists:

- **The resolver is only built for the human table.** `--output json`/`yaml`/
  `text` and `--query` keep the raw references *and* pay for no extra request.
- **Orphans keep the raw UUID** so a deleted referenced object does not break
  the listing.

The relationships are resolved by the shared [`internal/resolve`](../internal/resolve/resolve.go)
helper (built on the SDK v2 typed services, plus the SDK REST client for the
endpoints they don't wrap yet, such as `vm-templates`). See
[usage](usage.md) for each resource's detail view and example output.

## Performance

The rules above define a **cost model**, not just a style: the human views pay
a constant number of batch requests, independent of the number of rows, and
the machine output pays nothing at all. The unit tests pin part of that (each
`list_detail_test.go` counts the requests its fake server receives and asserts
the batch cost is exactly one `GetAll`), but when touching `internal/resolve`
or a `list`/`get` renderer it is worth re-verifying the model end-to-end with
the recipe below.

### Invariants to preserve

- `--output json`/`yaml`/`text` and `--query` must emit the raw data with
  **exactly one HTTP request** (the main fetch). The resolver is not even
  built for these formats — do not change that condition in the `list`
  commands.
- The default human `table` view costs **`1 +` one batch `GetAll` per
  referenced kind** — `host list` → 2 requests (hosts + pools), `vm list` → 3
  (vms + hosts + pools), `pbd list` → 4 (pbds + hosts + srs + pools) —
  **whatever the number of rows**. Never one lookup per row.
- Wall time grows with the **payload** of the fetched collections, not with
  the row count. A 500-row listing must not be meaningfully slower than a
  5-row one.

### How to measure it (old vs new, through a counting proxy)

Run two builds (the base, e.g. `origin/main`, and the change) against the
xo-api-sim simulator (started as in the
[functional tests section](#functional-tests-against-the-simulator)), fronted
by the request-counting proxy from [`ci/perf/proxy.py`](../ci/perf/proxy.py),
which forwards every request verbatim and appends one `METHOD /path` line per
request to `./reqs.log`:

```sh
git worktree add /tmp/xo-base origin/main
(cd /tmp/xo-base && go build -o /tmp/xo-base/bin/xo ./cmd/xo)
go build -o /tmp/xo-new/bin/xo ./cmd/xo

python3 ci/perf/proxy.py 3002 3001 &      # counter in front of the sim

export XOA_CONFIG_FILE=/tmp/xo-perf-config
export XOA_ENDPOINT=http://localhost:3002
export XOA_TOKEN=test-token

: > reqs.log
/tmp/xo-new/bin/xo vm list > /dev/null
wc -l < reqs.log                           # requests made by that invocation
cat reqs.log                               # and which endpoints they hit
```

Repeat per command and per output format, and compare old vs new. To make an
N+1 regression visible, scale the sim fixtures first: append a few hundred
rows to the listed collection (`src/fixtures/vms.json`), copy the fixtures
into `dist/fixtures/` (the sim serves from `dist/`), and restart it. The
table view's request count must not move with the row count.

### Reference numbers (xo-api-sim, 500 VMs / 30 hosts / 2 pools)

Measured through the counting proxy; the millisecond figures are orientation
only (local loopback, tiny payloads) — the **request counts are the
invariant**:

| Invocation | Before (main) | After (list) |
| ---------- | ------------- | ------------ |
| `vm list` — 500 rows, table | 1 req, ~35 ms | 3 req, ~90 ms |
| `vm list --output json` | 1 req, ~44 ms | 1 req, ~44 ms (unchanged) |
| ``vm list --query '[?power_state==`Running`].name_label'`` | 1 req | 1 req (unchanged) |
| `host list` — 30 rows, table | 1 req | 2 req (+1 batch) |
| `pbd list` — 11 rows, table | 1 req | 4 req (+3 batches) |

### Known trade-off

The batch `GetAll` fetches the **whole collection** of the referenced kind
(`fields=*`), even when only a few of its objects are actually referenced —
constant in request count, but the payload grows with farm size. That is the
price of the anti-N+1 design (the same model the XO web UI uses), and it is
fine for the kinds lists reference today (hosts, pools, SRs are small
collections). If a referenced kind ever grows large (thousands of objects),
the follow-up is to restrict the batch fetch to the fields it needs
(`id`, `name_label`) rather than reverting to per-row lookups.

## SDK v2: what we build on

This is the result of inspecting `xenorchestra-go-sdk` (pinned in `go.mod`)
before implementing each command, per the SDK-first rule in
[AGENTS.md](../AGENTS.md). The AGENTS.md *SDK versioning* checklist requires
this section to be refreshed whenever the SDK is upgraded.

### Module layout

The SDK is a **single Go module** (`github.com/vatesfr/xenorchestra-go-sdk`,
pinned at `v1.20.0` in `go.mod`) that contains **both** APIs:

| Path | API | Used by xo-gocli |
| ---- | --- | ---------------- |
| `client/`, `pkg/services/jsonrpc/` | v1: JSON-RPC over WebSocket | **never** |
| `v2/` | v2: REST client + typed services | **yes — the only API layer** |
| `pkg/config/` | shared config type | yes (via `v2`) |
| `pkg/payloads/` | shared REST response types | yes (returned by the typed services) |

There is no `v2/go.mod`: `v2` is a plain subdirectory, so the import path
`github.com/vatesfr/xenorchestra-go-sdk/v2` is a subpackage of the v1 module
— the `/v2` suffix is a directory name, not a Go major-version path. The
v1.20.0 version number is the *module* version, not the REST API version
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

### Two entry points

1. **`v2.New(cfg) → library.Library`** — the typed facade. Returns
   `VM()`, `Host()`, `Pool()`, `SR()`, `Network()`, `Task()`, `VDI()`, `VBD()`,
   `PBD()`. Every service method is a REST call through the internal
   `v2/client` (`client.TypedGet` & co.), returning `pkg/payloads` structs.
   This is what `internal/cli.NewClient` builds and what most commands use.
2. **`v2/client.New(cfg) → *client.Client`** — the raw REST client. The SDK
   *exports* `HttpClient`, `BaseURL` and `AuthToken` precisely so callers can
   reach endpoints the SDK does not (yet) wrap. `internal/cli.NewHTTPClient`
   is built on it and powers `xo rest`, `xo token`, `xo template`, `xo task`
   and the VM PATCH in `xo vm update`. That is the documented escape hatch,
   not a second REST client.

### Authentication & transport

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

### Request model (as the SDK sends it)

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

### Known SDK gaps the CLI works around

| Gap | CLI workaround |
| --- | -------------- |
| `VM().Update` returns `not yet implemented` | `xo vm update` sends the PATCH itself via the exported `*client.Client` (documented in `vm/update.go`; to contribute upstream) |
| No typed VM XVA/OVA import/export | `xo vm export` / `xo vm import` stream through the exported `*client.Client` against `GET /vms/<id>.<format>` and `POST /pools/<pool>/vms` (documented in `vm/xva.go`; to contribute upstream) |
| No typed service for `vm-templates`, tasks or user tokens | `TypedGet` directly (`xo template`, `xo task`, `xo token`) |
| `Task().Wait` / `WaitWithTimeout` unmarshal into `payloads.Task`, whose `Result` is a struct — but XO sometimes returns a task `result` as a **plain string** (see `TestTaskGetStringResult`), so `Get` (and therefore `Wait`) fails to unmarshal such tasks and loops to the deadline; `Wait` also does not treat `interrupted` as terminal (a server-side interruption makes the poll loop spin forever — filed as [vatesfr/xenorchestra-go-sdk#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121)) | `xo task wait` polls with `client.TypedGet` (the same single SDK boundary as `task get`) and stops on `success`/`failure`/`interrupted` (documented in `task/wait.go`; the SDK result type should be contributed upstream). The pool-maintenance commands go through the SDK's synchronous wait, so they inherit the hang until #121 lands; they now bound it with a wait-deadline `--timeout` flag and Ctrl+C (S3) |
| `users/me` 307-redirects to the user id | handled by `net/http` following the redirect; `doTokensRequest` relies on 307 body replay for POST |
| `v2` package `init()` runs `gotenv.Load()` (reads a `.env` in the CWD) | harmless: we build the config with `NewWithValues`, which reads no env vars |
| Server XO version is not exposed by the REST API (v0) — `GET /ping` only returns `{result, timestamp}` and the `xoa` REST controller has no version route | `xo version` prints the **CLI** version offline (like `aws version`); the server version exists only as the legacy JSON-RPC `getServerVersion` method, which the CLI never calls (v1 is forbidden by AGENTS.md) |

### Which command uses which SDK surface

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

## Roadmap

### Done

- `xo configure` + named profiles (with environment overrides)
- `list` / `get` for `vm`, `host`, `pool`, `sr`, `network`, `task`, `template`, `token`
- **`get` detail views** for `vm`, `vbd`, `pbd`, `vdi`, `sr`, `pool`, `network`, `host`, `template` and `task` (see the [`list` vs `get` convention](#list-vs-get-the-detail-view-convention)): each is a key/value sheet that resolves relationships to names at a constant, anti-N+1 cost, with raw-id fallback; machine output (`--output json`/`yaml`/`text`, `--query`) is unchanged
- **`list` reference columns resolved to names** for `vm`, `sr`, `pool`, `host`, `network`, `vdi`, `vbd`, `pbd`, `template` and `vm vdis`: the columns that reference another object (`MASTER`, `SR`, `VM`/`VDI`, `POOL`, `CONTAINER`) show the object's name instead of its UUID, resolved with one batch per referenced kind (constant cost, orphans keep the raw UUID); `--output json`/`yaml`/`text` and `--query` keep the raw references and make no extra request
- `task wait` (blocks until a task reaches a terminal state; exit status reflects the outcome)
- `task abort` (asks Xen Orchestra to interrupt a pending task; confirmation + `--yes`; pre-checks existence and that the task is still pending, so a finished task is rejected with a clear error)
- `--wait` on the asynchronous actions (`vm start/stop/reboot/pause/unpause/suspend/resume/snapshot`, `vbd connect/disconnect`, `pbd plug/unplug`, `sr scan/reclaim-space`; plus `$XOA_WAIT` for scripts): blocks until the action's task completes, then renders it (shared `internal/taskwait` package, reused by `task wait`)
- `vm vdis` (per-VM VDI listing)
- `sr scan` / `sr reclaim-space` (SR maintenance actions)
- VM lifecycle: `create`, `start` (host pinning), `stop` (clean/hard), `reboot` (clean/hard), `pause`/`unpause`, `suspend`/`resume`, `snapshot`, `delete`, `export`/`import` (XVA/OVA), `update`, `tag add/remove`
- Pool maintenance: `rolling-update`, `rolling-reboot` and `emergency-shutdown` (confirmation + `--yes` for the two destructive ones; all synchronous). Each now takes a `--timeout` *wait* deadline (unbounded by default; shadows the global per-request `--timeout`, like `task wait`) so a long or stuck operation can be bounded — see the [Known SDK gaps](#known-sdk-gaps-the-cli-works-around) row on `Task().Wait` / `interrupted`
- `tag add/remove` on `host`, `pool`, `sr` and `network` (any taggable resource, not just VMs)
- `network create` / `create-internal` / `create-bonded` / `delete`
- `vdi list/get/create/delete` + `migrate`/`tag add-remove`/`export`/`import` and `vbd list/get/create/delete` + `connect`/`disconnect` — VDI and VBD are first-class resources (enables the "add a disk to a VM" use case)
- `pbd list/get` + `plug`/`unplug` — PBD is a first-class resource (`Plug`/`Unplug` are asynchronous and return a task id)
- `xo rest` raw REST escape hatch on top of the SDK v2 HTTP facilities
- `-d`, `--debug` global flag (+ `$XOA_DEBUG`): reveals the raw SDK/API error and the resolved profile/endpoint behind a concise failure
- `--timeout` global flag (+ `$XOA_TIMEOUT`): overrides the SDK's 30-second HTTP client timeout for long-running operations
- Machine-output contract (road-to-v1 S4): `--json` global flag (a shortcut for `--output json`), a resolvable default output via `XOA_DEFAULT_OUTPUT` and a per-profile `output:` config value (precedence `--output` > `--json` > env > profile > `table`), whole numbers rendered as plain integers in YAML (no `2.147483648e+09`), and the JSON/YAML contract documented in the usage page
- Human-view conventions unified (road-to-v1 S5): booleans are `yes`/`no` everywhere, `VERSION` means the platform version on both `host list` and `pool list` (`CORES`/`SOCKETS` added to `host list`), `CONTAINER` for the container column on `vm`/`sr`, the constant `network TYPE` column dropped, zero sizes render `0 B` (list) / `-` (detail sheets, no dangling `Label:`), `vdi get` "Attached to" capped at 10 names, empty lists print the header plus `No <resources> found.`, and `vm list` gained the `IP` column
- `POWER STATE` reflects the operation in flight (`vm list` column, `vm get` header): while a lifecycle task runs, the SDK v2 `current_operations` predicates (`IsStarting` / `IsShuttingDown` / `IsRebooting`, SDK v1.20.0) render `Starting` / `Shutting down` / `Rebooting` instead of the still-lagging raw `power_state`; display-only — the server-side `--power-state` filter and machine output keep the raw fields
- Wait bounds decided and documented (road-to-v1 S3): **no default wait deadline** — every wait (`task wait`, `--wait`, pool maintenance) runs as long as the operation does and Ctrl+C cancels it (the command context propagates through the SDK to the HTTP layer); each request stays bounded by the global HTTP client timeout. Pool maintenance gained its wait `--timeout` deadline, `task wait`'s `--timeout` keeps meaning the wait deadline (shadowing the global flag, as documented), and `vm`/`vdi` `export`/`import` help now note that the global `--timeout` (30 s default) bounds the single transfer request; the contract is documented in the "Timeouts and waiting" section of the usage page
- Shell completion generated by cobra (`xo completion bash|zsh|fish|powershell`), documented in the usage page

The remainder is organized in four layers. The deeper a layer, the more it
depends on SDK work (see the
[architecture rules](../AGENTS.md): nothing bypasses `xenorchestra-go-sdk/v2`).

### Layer 1 — already in the SDK, not exposed by the CLI yet

Every item below is a command (or a small set of commands) on top of an
existing, implemented SDK service method — the same pattern as the current
commands. No SDK work is required.

- — (empty: every SDK v1.20.0 service method that maps to a single CLI command
  is now exposed; `xo task abort` was the last one to be covered)

Each item added here reduces the need for `xo rest`.

### Layer 2 — complete SDK services, no CLI resource yet

Full resources following the usual `list / get / …` shape, built on an
implemented SDK service (verified in `v1.20.0`):

- — (empty: every SDK v1.20.0 service now has a matching CLI resource;
  `xo pbd` was the last one to be covered)

### Layer 3 — in the REST API, not in the SDK yet

These resources exist in the Xen Orchestra REST API but have no typed SDK
service. They require an upstream contribution (or the exported
`*client.Client` in the meantime, per the SDK-first rule). None of them can
be built on the SDK v2 as it stands:

- `xo snapshot` — VM snapshots (`/vm-snapshots` list/get/delete +
  `revert_snapshot`). Note: *creating* a snapshot is already in the SDK
  (`VM().Snapshot`, exposed as `xo vm snapshot`); it is the snapshot
  resource itself that is missing.
- `xo backup` — `backup-jobs`, `backup-logs`, `repositories`, `archives`
- `xo vm` actions: `clone`, `migrate` (REST actions not in the SDK)
- Resources: `vif`, `user`, `group`, `acl-role`, `acl-privilege`, `server`
  (CRUD + `connect`/`disconnect`), `alarm`, `message`, `task clear`
  (`DELETE /tasks`)
- Read-only: `dashboard`, host `stats`/`logs`

### Layer 4 — not in the SDK at all

- `xo watch` (live events): `GET /events` is a **SSE** stream and the SDK has
  no SSE support. Per the architecture rules this goes through a SDK
  contribution, not an in-CLI `net/http` stream.

### CLI-side polish (no SDK dependency)

- Widen `xo vm create` beyond `--memory`/`--boot` (affinity, autoPoweron,
  clone, cloudConfig, install, extra VDIs/VIFs — all in `CreateVMParams`)
- Widen `xo vm update` beyond name/description/tags

### SDK bug to verify upstream

`VM().Restart()` POSTs to `/vms/<id>/actions/restart`, which does not appear
in the REST API OpenAPI spec (the reboot actions are `clean_reboot` /
`hard_reboot`). The CLI is not affected (`xo vm reboot` uses the correct
actions), but the SDK method should be checked against a live instance and
fixed upstream if broken.

### SDK bugs confirmed & fixed upstream

- **`payloads.VBD.Position` typed as `StringifiedInt`** (broke every
  `VBD().Get` / `GetAll` against real instances:
  `failed to unmarshal response strconv.Atoi: parsing "xvdb": invalid syntax`).
  The REST API exposes the XAPI `userdevice` under the `position` key as a
  string whose content is data dependent (a numeric index on some stacks, a
  device name such as `xvdb` / `cd0` on others); the SDK's own v1 client
  already types it as `string`. Fixed upstream as a `string` (PR
  [vatesfr/xenorchestra-go-sdk#119](https://github.com/vatesfr/xenorchestra-go-sdk/pull/119),
  regression tests added in the SDK) and picked up here via the `replace`
  pin described in [Module layout](#module-layout) until it ships in a
  release.

## License

This project is licensed under the [MIT License](https://opensource.org/licenses/MIT),
the same license as the Xen Orchestra Go SDK.

