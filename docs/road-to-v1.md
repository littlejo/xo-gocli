# Road to v1.0.0

Status snapshot and the minimal checklist for releasing `xo` v1.0.0.

> **Snapshot**: `main` at `7d4f9ed` (2026-10-06). B1, B2, S1–S7 are all done
> and merged; the C1 counter-expertise fix (template id for `vm create`) is
> in review; the release-day steps remain. The
> rest of this page is based on
> the original read-only audit of the full codebase (every command file, the
> output layer, the test suite, the docs and the release pipeline), with the
> built binary exercised directly to confirm the key findings. A second,
> independent counter-expertise pass (2026-10-05) re-verified everything
> against the real binary, a fake XO server and the upstream Xen Orchestra
> REST source; it found five additional items (C1–C5, see
> [Counter-expertise findings](#counter-expertise-findings-2026-10-05)) with
> an initial verdict of **READY AFTER SMALL FIXES** — C1 is now fixed, which
> clears the only remaining must-fix for the tag. As items are fixed, cross
> them off in the [release checklist](#release-checklist) and update the
> snapshot line.

## Table of contents

- [Where we are](#where-we-are)
- [Blockers](#blockers)
  - [B1: `xo rest` panics on every invocation](#b1-xo-rest-panics-on-every-invocation)
  - [B2: no `LICENSE` file](#b2-no-license-file)
- [Should fix before v1.0.0](#should-fix-before-v100)
  - [S1: pin the scripting contract with process-level tests](#s1-pin-the-scripting-contract-with-process-level-tests)
  - [S2: translate SDK/Go errors before showing them](#s2-translate-sdkgo-errors-before-showing-them)
  - [S3: bound the unbounded waits](#s3-bound-the-unbounded-waits)
  - [S4: machine output and the documented contract](#s4-machine-output-and-the-documented-contract)
  - [S5: unify the human output conventions](#s5-unify-the-human-output-conventions)
  - [S6: README accuracy](#s6-readme-accuracy)
  - [S7: two open UX decisions](#s7-two-open-ux-decisions)
- [Counter-expertise findings (2026-10-05)](#counter-expertise-findings-2026-10-05)
- [Release-day process (manual steps)](#release-day-process-manual-steps)
- [Tracked follow-ups (not release-blocking)](#tracked-follow-ups-not-release-blocking)
- [Deferred to v1.1+](#deferred-to-v11)
- [Release checklist](#release-checklist)

## Where we are

The product is technically close to releaseable. Verified at snapshot time:

- **Coverage.** 13 command groups, ~60 subcommands: `vm` (17 ops including
  create/update/export/import/snapshot), `host`, `pool` (incl. rolling-update,
  rolling-reboot, emergency-shutdown), `sr` (scan, reclaim-space), `network`
  (create/create-internal/create-bonded/delete), `template`, `task`
  (wait, abort), `vdi` (migrate, export/import), `vbd` (connect/disconnect),
  `pbd` (plug/unplug), `token`, plus `rest`, `configure`, `version`,
  `completion`. Every SDK v1.20.0 service method that maps to a single command
  is already exposed (roadmap Layers 1–2 are empty).
- **Architecture.** One API boundary: imports are limited to the SDK's `v2`
  and shared `pkg/` packages — no legacy v1 client import anywhere (grep
  verified). `internal/resolve` is the single anti-N+1 helper shared by every
  `get`/`list` renderer.
- **No N+1 anywhere.** Request counts were derived per command: table views
  cost `1 + one batch GetAll per referenced kind`, constant in the row count;
  `--output json/yaml/text` and `--query` make exactly the main request
  (the resolver is not even built). The SDK's deprecated `VM().List()` —
  N+1 by construction — is not called by any command.
- **Tests.** ~490 unit tests pass; fakes enforce auth (401) and unknown paths
  (404); request paths/bodies and error wording are pinned; anti-N+1 cost is
  asserted per resource; secrets are verified masked in table and JSON output.
- **Hygiene.** `gofmt`, `go vet`, golangci-lint 2.14.0: clean.
- **Docs.** `usage.md` is in sync with the branch; every leaf command has a
  `Long` description and `Examples`.
- **Release pipeline.** `ci.yml` (lint + tests + a `functional` job against
  xo-api-sim), `version.yml` (semver from conventional commits), `release.yml`
  (GoReleaser, amd64/arm64, checksums).

What keeps it from a v1.0.0 tag is the small list below — the two hard
blockers are resolved (B1, B2), as are S1–S7 (all merged, S3 in `cde603c`).
The independent counter-expertise pass of 2026-10-05 confirmed the state and
added five findings (C1–C5); **C1 is fixed** (template id for `vm create`),
leaving C2–C5 as tracked, non-blocking follow-ups.

## Blockers

### B1: `xo rest` panics on every invocation

Every form — `xo rest get …`, `xo rest delete …`, `xo rest --help`, even bare
`xo rest` — crashes with a Go stack trace and exit code 2:

```
panic: unable to redefine 'd' shorthand in "rest" flagset: it's already used for "data" flag
```

The root command owns the shorthand `-d` for the global `--debug` flag
(`internal/commands/commands.go`, added with the `--debug` feature), while
`rest` registers `-d` for `--data` (`internal/commands/rest/rest.go`). Cobra
panics in `mergePersistentFlags` when the flag sets are merged.

Key facts:

- It is **broken on `main` too**, not just this branch — the `--debug` global
  flag landed after `rest` existed, so the collision predates the `list`
  branch.
- The test suite is green because `rest_test.go` builds its own minimal root
  that does *not* define the `-d` global shorthand, so the real root's flag
  merge is never exercised. This is exactly the class of regression the
  process-level test in S1 is meant to catch.
- `rest` is a roadmap "Done" item and is documented in `usage.md`, so users
  are expected to use it.

Fix (one line + one test):

- drop the `-d` shorthand from `--data` (keep `--data`; the `rest` help text
  already says "use `--data -` to read stdin"), or rename the flag;
- add a test that builds the command on the **real** root (or an `os/exec`
  smoke test of `xo rest --help`) so any future shorthand collision fails CI
  instead of shipping.

### B2: no `LICENSE` file

`README.md` and `docs/development.md` both claim MIT, but no `LICENSE` file
exists in the repository. Without one the code has no explicit license (the
default is "all rights reserved"), which is unacceptable for a public v1.0.0.

Fix: add the standard MIT `LICENSE` file with the project's copyright line.

## Should fix before v1.0.0

### S1: pin the scripting contract with process-level tests

The "AWS-CLI-like, scriptable" promise rests on behaviors the current suite
never exercises (all tests call in-process APIs; none run the compiled
binary). The `rest` panic (B1) shipped through a fully green suite for that
reason. Add, at minimum:

1. **Exit codes + stderr discipline** via `os/exec` on the built binary:
   success → 0; API error → 1 with `Error: …` on **stderr** and nothing on
   stdout; usage error → 1.
2. **401 / rejected token** — the most common real-world failure; no test
   ever sends a wrong token (every fake server verifies the cookie and every
   test sends the right one).
3. **Context cancellation** — `Ctrl+C → command → SDK → HTTP` is an
   AGENTS.md requirement; no test uses a cancellable context.
4. **`--profile` end-to-end** — seed a profile in the config file and run a
   resource command (e.g. `vm list --profile lab`); today only env vars are
   tested.
5. **Golden JSON** — pin the exact shape of `vm list --output json` and
   `vm get <id> --output json` (field set, not just presence) so a rename or
   accidental extra field cannot pass silently.

### S2: translate SDK/Go errors before showing them

AGENTS.md says users should never need to understand `json.Unmarshal` to
diagnose a failure. Only the 404 path is currently clean; the rest leaks Go
internals (all reproduced against a live-looking server):

| Situation | What the user sees today |
| --- | --- |
| HTTP timeout | `context deadline exceeded (Client.Timeout exceeded while awaiting headers)` |
| Unreachable endpoint | `dial tcp 127.0.0.1:8901: connect: connection refused` |
| Malformed task result | `json: cannot unmarshal string into Go struct field Task.result of type payloads.Result` |
| Bad id in an edge case | `uuid: incorrect UUID length 4 in string "vbd1"` |

Add a small mapping layer (timeout → `timed out after 30s`, network dial
failure → `cannot reach <endpoint>`, SDK unmarshal → generic API error) and
keep the raw detail behind `--debug`, which already exists. While in there,
replace the not-found detection's `404` **substring** match
(`internal/cli/client.go`) with a check on the actual HTTP status.

### S3: bound the unbounded waits

Three places can block for a very long time with no documented exit:

- **`xo task wait`** polls every 2 s with **no default deadline**; a stuck
  task blocks until Ctrl+C. `task wait --timeout` exists but *shadows* the
  global `--timeout` (it is the wait deadline, not the per-request one) — the
  help text should make the difference explicit.
- **`pool rolling-update / rolling-reboot / emergency-shutdown`** block for
  the whole operation, issuing a task `GET` every 2 s (hundreds of requests
  on a large pool), with no `--timeout` flag. Additionally, the SDK's
  `task.Wait` does not treat `interrupted` as terminal, so a server-side
  interruption can hang the command until the context dies — verify against a
  live instance and file the SDK issue (the CLI's own `taskwait` loop already
  treats `interrupted` as terminal; the SDK-side one does not).
- **`vm export` / `import` and `vdi export` / `import`** stream over a single
  request bounded by the global `--timeout` (**30 s default**); a large XVA
  times out silently in practice. One line of help text on those commands.

Decision to make before the tag: a default wait deadline (e.g. 10 min) or
"Ctrl+C, documented". Either is defensible; *nothing documented* is not.

**Resolved (branch `s3-bounded-waits`):**

- *Decision: "Ctrl+C, documented" — no default wait deadline.* A fixed
  cutoff would report a timeout for an operation that is legitimately still
  running (or has already completed) server-side, leaving the caller unsure
  of the real state; that is a worse failure than a wait that ends when the
  task ends. Every wait therefore runs as long as the operation does, and
  Ctrl+C cancels it — the command context (wired with
  `signal.NotifyContext` in `cmd/xo/main.go`) propagates through the SDK to
  the HTTP layer. Each individual request is still bounded by the global
  HTTP client timeout, so a broken connection can never hang a poll
  forever. The contract is documented in `usage.md` (new "Timeouts and
  waiting" section, which replaces the inaccurate "Request timeout" one:
  the global `--timeout` bounds a single request — the whole transfer for
  `export`/`import`, each poll for the waits — not the duration of a poll
  loop).
- *Pool maintenance is now boundable.* `rolling-update`, `rolling-reboot`
  and `emergency-shutdown` gain a `--timeout` flag: the *wait* deadline,
  default unbounded, shadowing the global per-request flag — the same
  convention as `task wait`. `runAction` derives a timed context around the
  SDK call, and a wait cut off by the deadline (or by Ctrl+C) is reported as
  such ("…did not complete within 30m0s (raise --timeout, or track the task
  with 'xo task wait')" / "…was cancelled") instead of leaking the SDK's raw
  "task wait timed out" string. Test: a backing task stuck in
  `interrupted` (never terminal for the SDK's own loop) with
  `--timeout 1s` returns the concise deadline error in ~2 s instead of
  hanging.
- *Export/import help note.* `vm export`/`import` and `vdi export`/`import`
  now say that the transfer is a single HTTP request bounded by the global
  `--timeout` (30 s default) and that it should be raised
  (`--timeout` / `$XOA_TIMEOUT`) for large archives over a slow link.
- *`task wait` help already made the shadowing explicit* (wait deadline vs
  global per-request timeout, `$XOA_TIMEOUT` for the latter) — no change.
- *`interrupted` hang: verified and filed upstream.* The SDK's
  `task.Wait` (v1.20.0, `pkg/services/task/service.go`) stops its poll loop
  only on `success`/`failure`; a server-side interruption makes it spin
  forever. Verified in the SDK source (no live instance available in this
  environment, `XOA_TEST_URL` unset); the CLI's own `internal/taskwait` loop
  already treats `interrupted` as terminal, and the new pool `--timeout` /
  Ctrl+C bounds even that path. Filed as
  [vatesfr/xenorchestra-go-sdk#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121)
  (suggested one-line fix included).

### S4: machine output and the documented contract

- **`--json` does not exist** — `xo vm list --json` fails with `unknown
  flag`. AWS parity is part of the positioning; add it as a hidden/alias
  boolean that selects `-o json` (or document `-o json` prominently if you
  decide against the flag).
- **YAML numbers render in exponent form** for large integers
  (`size: 2.147483648e+09` — verified) because the pipeline round-trips
  through `float64`; above 2^53 precision is also lost. Decide: document the
  YAML caveat or route YAML through a typed number marshaler.
- **Document the JSON contract.** `--output json` emits the SDK struct shape,
  not the API response byte-for-byte: `omitempty` zeros are dropped, struct
  zero values are added, fields the SDK payload does not model are absent.
  This is stable and intentional, but it is a v1 API — it needs one
  subsection in `usage.md`.
- **Mutation JSON embeds names, not ids** (`{"action":"delete","vm":"web-01"}`).
  Defensible (the name is what the user typed), but it is human data in a
  machine stream — document it, or switch to the id.

**Resolved (branch `s4-machine-output`):** `--json` added as a global boolean
shortcut for `--output json` (an explicit `--output` wins); YAML now converts
whole numbers to `int64` before marshaling (`size: 2147483648`), with the
`> 2^53` precision caveat documented — `Normalize`/JMESPath stay on `float64`
because the engine cannot compare `int64`; the JSON contract and the
mutation-JSON name-vs-id decision (keep the name, document it) are documented
in the `Output & querying` section of `usage.md`. Default output became
resolvable: `--output` > `--json` > `$XOA_DEFAULT_OUTPUT` > profile `output:`
> `table`.

### S5: unify the human output conventions

The table/detail renderers are consistent in structure but not in
convention. Each item is a small, local fix; one pass over the renderers:

| Inconsistency | Where |
| --- | --- |
| Booleans rendered two ways: `true`/`false` vs `yes`/`no` | pool `HA`, template `DEFAULT` vs vbd/pbd `ATTACHED` |
| `PLATFORM` means different things: host = raw version, pool = `platform_version`; and `host list` ≠ `host get` (which composes brand+version+build) | `host/list.go` vs `pool/list.go` vs `host/get.go` |
| Zero-byte sizes: blank cell (vm/host/template MEMORY) vs `0 B` (sr/vdi SIZE/USAGE) | the `memoryText`/`sizeText` helpers |
| Same concept, two column names: `HOST/POOL` (vm) vs `CONTAINER` (sr) | `vm/list.go`, `sr/list.go` |
| `network list` `TYPE` column is the constant `network` on every row (SDK `ResourceType`) — redundant | `network/list.go` |
| `sr`/`vdi` `get` can print a bare `Label:` with no value when size/usage is 0 (the one place the detail sheet breaks its own "always complete" rule) | `sr/get.go`, `vdi/get.go` |
| `vdi get` "Attached to" lists names **uncapped** (every other collection line caps at 10) | `vdi/get.go` |
| **Empty collection in table mode prints nothing at all** (no header, no message) while json/yaml emit `[]` — a silent no-op is the worst first-run experience | `internal/output/output.go` — print the header plus `No <resources> found.` |

Also consider the two missing columns a first user expects: `host list` has
no CPU column while `pool list` shows CORES/SOCKETS (inconsistent), and
`vm list` has no IP column (`mainIpAddress` is already in the payload).

**Resolved (branch `s5-human-output`):** booleans are `yes`/`no` everywhere
(pool `HA`, template `DEFAULT` joined vbd/pbd `ATTACHED`); `VERSION` is the
platform version on both `host list` and `pool list` (the `get` sheets keep
the `Platform` label for the brand+version+build composition); `CONTAINER`
is the container column on both `vm list` and `sr list`; the constant
`network TYPE` column is dropped; zero sizes render `0 B` in list tables and
`-` in detail sheets (no dangling `Label:` line); `vdi get` "Attached to" is
capped at 10 names like every other collection line; empty lists print the
header plus `No <resources> found.` (`Table.Empty`, machine output
unchanged); `vm list` gained the `IP` column and `host list` gained
`CORES`/`SOCKETS`.

### S6: README accuracy

- `--insecure` is listed as a general feature, but it only exists as
  `xo configure --insecure` / `$XOA_INSECURE` — there is no per-command flag.
- No supported-platforms statement (amd64/arm64 only, 64-bit because of the
  SDK; Windows binary exists as a zip but `install.sh` refuses Windows).
- No overview of the command groups; a first-time user must open `usage.md`
  to learn what exists.

**Resolved (branch `s6-readme-accuracy`):** the `--insecure` feature bullet
now names its real surface (`xo configure --insecure` per profile, or
`$XOA_INSECURE`); the platform bullet states Linux/macOS/Windows, amd64 and
arm64 only (the SDK does not compile on 32-bit), and that Windows ships as a
`.zip` while `install.sh` covers Linux/macOS; a new "Commands" section lists
every resource group and its operations.

### S7: two open UX decisions

1. **`vm stop --hard` asks for confirmation; `vm reboot --hard` does not** —
   same action family, different risk treatment. Decide and make it consistent
   (or document the rationale).
2. **Double auth login with username/password profiles.** Five commands
   (`vm update`, `vm export`, `vm import`, `vm vdis`, `task abort`) construct
   two SDK clients, so a password profile performs `POST /auth/login` twice
   per invocation (zero cost with a token profile). Sharing one client is a
   small refactor; at minimum, decide whether it ships in v1.0.0 or v1.1.

**Resolved (branch `s7-ux-decisions`):**

1. *Confirmation.* `vm stop` no longer asks for confirmation. Rationale: a
   stopped VM is brought back by `xo vm start`, so `stop` is reversible by
   its counterpart — exactly like the other power actions that already never
   prompt (`reboot`, `pause`, `suspend`, …). Keeping the prompt on `stop`
   alone made the power family inconsistent and interrupted the normal
   start/stop/reboot loop; this matches the AWS CLI, where
   `stop-instances` and `reboot-instances` are both non-interactive. `vm
   delete` and the pool-maintenance actions keep their confirmation (they are
   not reversible). `--yes`/`$XOA_YES` are unchanged.
2. *Double login.* Decided **v1.1**: a real fix requires the SDK to expose a
   way to build the typed services around a single already-authenticated REST
   client (it does not today — `xov2.New` always logs in when given
   username/password, and the `XOClient` facade is unexported). That is an
   SDK contribution + re-pin of the fork + a refactor of `NewResolver` and the
   five commands, which is too much surface to change immediately before the
   v1.0.0 tag. In the meantime the caveat is documented in `usage.md`
   (Configuration: token profiles are recommended), and a tracking issue is
   opened. Note the audit's "five commands" under-counts the surface: every
   `list`/`get` table goes through `NewResolver`, which also builds two
   clients, so the duplicate login touches most commands — reinforcing the
   "token profiles" guidance.

## Counter-expertise findings (2026-10-05)

A second, independent audit pass (verifying the original audit's claims
rather than trusting them) built the binary and exercised it against a fake
Xen Orchestra REST server with a per-path request counter, and cross-checked
the API contract against the upstream `vatesfr/xen-orchestra` REST source
(`@xen-orchestra/rest-api`, `@vates/types`) and against the pinned
xo-api-sim ref (`ee5e2a9`) used by the CI `functional` job. Confirmed
correct: no N+1 anywhere (request counts), stdout/stderr discipline and exit
codes, confirmations (non-tty refusal, `--yes`/`XOA_YES`), `--wait`/
`task wait` outcome semantics (failure and `interrupted` both exit non-zero,
task on stdout), the JSON/YAML contract (int64 sizes, no exponent form),
token masking (`create` prints in full once, `list` masks table and JSON),
no panics across a broad sweep, Ctrl+C propagation (≤0.1 s), and — notably —
`vm update`'s camelCase PATCH body is **correct**: the real XO route is
documented with exactly that shape (`{"nameLabel": …, "nameDescription": …}`).

Five new findings:

### C1: `vm create --template` rejects the id that `xo template list` prints — HIGH — **FIXED**

**Resolved:** `--template` now accepts both the bare template UUID and the
composite `<poolId>-<templateUuid>` id that `xo template list` prints,
reducing the composite form to the bare UUID that `create_vm` wants
(`parseTemplateID` in `internal/commands/vm/create.go`). The help text no
longer claims the template is "referenced by its UUID, as returned by
'xo template list'"; it now names both accepted forms. The documented flow
(copy the printed id → paste it in) works. Covered by
`TestVMCreateCompositeTemplateID` (asserts the create_vm body carries the
bare uuid) and `TestVMCreateInvalidTemplateID`. The existing integration test
`lifecycle_integration_test.go` now exercises the working flow, since it
feeds the `id` from `template list` into `vm create`.

The original finding, for the record:

- `xo template list` prints the REST `id` field, which on real XO is the
  **composite** `poolId-templateUuid` (canonical form confirmed in the
  upstream controller's documented examples, e.g.
  `id: 'fe3d015b-…-7279a78a-…'`; `XoVmTemplate.id` is a branded composite
  while `uuid` is the bare form).
- `vm create --template` (`internal/commands/vm/create.go`) requires a
  **bare UUID** (`uuid.FromString`) — and that part is correct: the real
  `create_vm` wants `XoVmTemplate['uuid']`.
- But the help says the template is "referenced by its UUID, as returned by
  'xo template list'", so the documented flow (copy the printed id → paste it
  in) fails on real XO with
  `Error: invalid --template id "…" (expected a UUID)`.
- The CI cannot see this: the pinned xo-api-sim models templates with
  `id == uuid` (bare UUIDs), a different id model than production. Worse,
  the repo's own `lifecycle_integration_test.go` encodes the broken flow —
  it takes the first `id` from `template list --output json` and passes it to
  `vm create --template` — and only passes in CI because of the sim's model.
- Workaround today: `xo template list --output json | jq '.[].uuid'` (the
  real REST object carries the bare `uuid` field), but it is undiscoverable.

Fix applied: accepted both forms in `--template` (bare UUID, or composite
`poolId-uuid` with the trailing UUID extracted) and corrected the help text.
The `usage.md` examples already used the neutral `<template-id>` placeholder,
so they needed no change. Small change, one flow, done before the tag.

### C2: SDK-driven waits spin forever on a stuck `interrupted` task (`vm create`, `network create*`) — MEDIUM, tracked

Empirically confirmed against a fake whose task flips to `interrupted` and
stays there:

- `pool rolling-reboot`: **hangs** without `--timeout` (killed by an
  external watchdog); `--timeout 5s` produces the concise deadline error
  (`…did not complete within 5s (raise --timeout, or track the task with
  'xo task wait')`, exit 1); Ctrl+C cancels in 0.1 s. So the S3 mitigation
  works on the pool commands.
- Same task with `xo task wait`: returns in ~10 ms with
  `task … was interrupted` (the CLI's own `internal/taskwait` treats
  `interrupted` as terminal) — an **internal asymmetry**: the CLI's own wait
  is correct, the SDK's `task.Wait` (used by `vm create`, `network
  create`/`create-internal`/`create-bonded` and the pool maintenance) is not.
- `xo vm create` on a stuck create task: **hangs**, and `vm create` has no
  `--timeout`/wait-deadline flag at all — only Ctrl+C. Realistic trigger:
  the user interrupts with `xo task abort` (a shipped command) while a
  create/maintenance is in flight.

This is the known SDK gap of S3 ([vatesfr/xenorchestra-go-sdk#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121)),
now empirically verified (no live instance available). Accepted for v1.0.0
with the existing mitigations (Ctrl+C, pool `--timeout`). Two one-line doc
fixes are tracked with C4: the `usage.md` "Timeouts and waiting" section
lists the unbounded waits as "the pool …, `task wait`, and the `--wait`
flag" — `vm create` and `network create*` belong in that list too; and the
same sentence is worth putting in their `--help`.

### C3: the release workflow's `test` job is weaker than CI — LOW, tracked

`release.yml` runs only gofmt + vet + `go test ./...`, while `ci.yml`
additionally runs `go test -race ./...`, the integration tests and the
xo-api-sim `functional` job. `workflow_dispatch` can release any tag,
including one that never passed CI. Recommendation: mirror the CI gates (at
least `-race` + functional) in the release `test` job before the first
release.

### C4: `vm create --boot` still prints "Start it with: xo vm start …" — LOW, tracked

`renderCreatedVM` (`vm/create.go`) prints the start hint unconditionally;
with `--boot` the VM is created already running, so the hint is wrong.
One-line fix: skip it when `--boot` is set.

### C5: `usage.md` "Mutating commands (create, update, delete, start, …) emit a small result document, not the full object" — LOW, tracked

`vm create --output json` emits the **full VM object** (verified); the
`{"action": …, "vm": …, "task_id": …}` shape applies to the action commands.
Doc fix: exclude `create` (and check `update`) from that sentence.

Verdict of the pass: **READY AFTER SMALL FIXES** — C1 before the tag; C2–C5
are tracked follow-ups (C2's SDK half already tracked via #121).

## Release-day process (manual steps)

The normal flow ("push to main → auto tag + release") **cannot produce a
1.0**: the version bumper maps `feat` commits to a *minor* bump (merging the
`list` branch would tag v0.31.x) and has no major-bump path. v1.0.0 is a
hand-cut release:

1. Merge `list` → `main`; require a fully green CI, **including the
   `functional` job** (xo-api-sim end-to-end).
2. Push the tag **by hand**: `git tag -a v1.0.0 -m "…" && git push origin
   v1.0.0`. `release.yml` validates the strict `vMAJOR.MINOR.PATCH` shape and
   builds via GoReleaser.
3. **Publish the release.** GoReleaser publishes a *draft*; nothing is
   installable (`install.sh` hits `releases/latest` and fails with "no
   release found") until a human publishes it from the Actions/Releases page.
4. Release notes: the v1.0.0 headline (name resolution on `list`/`get` at a
   constant anti-N+1 cost; first-class vdi/vbd/pbd; `rest` escape hatch).
   The SDK dependency is now plain upstream `v1.20.0` (the `VBD.Position`
   fix, PR #119, shipped there), so no fork disclosure is needed.
5. Smoke-test `install.sh` against the published release on Linux amd64 and
   arm64, then run the README quick start verbatim.

## Tracked follow-ups (not release-blocking)

- **SDK fork pin — resolved (branch `sdk-v1.20.0`).** `go.mod` no longer
  replaces `github.com/vatesfr/xenorchestra-go-sdk` with the fork: PR #119
  (`VBD.Position` as string) shipped in upstream **v1.20.0** (released
  2026-10-05), so the `replace` was dropped and the `require` bumped to the
  plain upstream release. One CLI adaptation: `payloads.VM.BlockedOperations`
  and `payloads.VM.CurrentOperations` keys are now the `VMOperation` string
  type (PR #118), not `string`. No new command surface: the `v2/` services
  are unchanged between v1.19.0 and v1.20.0.
- **SDK `task.Wait` and `interrupted` — filed
  ([vatesfr/xenorchestra-go-sdk#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121)).**
  The SDK's wait loop only treats `success`/`failure` as terminal; the CLI
  works around it in `internal/taskwait` and now bounds the pool-maintenance
  wait with `--timeout` / Ctrl+C (S3). Resolved upstream once #121 lands.
- **Three duplicated request builders** on top of the SDK HTTP client
  (`rest.doRestRequest`, `vm/xva.xvRequest`, `token.doTokensRequest`), each
  re-implementing URL joining, cookie attachment and error formatting —
  consolidate into one helper to remove the drift surface.
- **Double login for username/password profiles** (issue #56): commands that
  build both the typed facade and the REST client (the resolver on every
  `list`/`get` table, plus `vm update/export/import/vdis` and `task abort`)
  send `POST /auth/login` twice. Decided in S7 to defer the fix to v1.1 — it
  needs the SDK to build the typed services around a single
  already-authenticated REST client. `usage.md` now recommends token
  profiles until then.
- **C2 (counter-expertise):** the SDK-driven waits (`vm create`,
  `network create*`, pool maintenance) spin until Ctrl+C on a task stuck in
  `interrupted`; the pool `--timeout` and Ctrl+C bound it, `task wait`
  (CLI-side) already returns on `interrupted`. Blocked on
  [vatesfr/xenorchestra-go-sdk#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121);
  until it lands, add `vm create` / `network create*` to the "unbounded
  waits" list in `usage.md` ("Timeouts and waiting") and in their help text.
- **C3 (counter-expertise):** mirror the CI gates (`go test -race`,
  integration, xo-api-sim `functional`) in the `test` job of `release.yml`,
  so a hand-dispatched release can never skip them.
- **C4 (counter-expertise):** `vm create --boot` should not print the
  "Start it with: xo vm start …" hint (one-line fix in `renderCreatedVM`).
- **C5 (counter-expertise):** `usage.md` "Mutation and action output" says
  mutating commands emit a small result document — but `vm create --output
  json` emits the full VM object; fix the sentence.

## Deferred to v1.1+

Deliberately *not* in the v1.0.0 scope (each is a real improvement, none is
required for a coherent release):

- Name-based lookup in `get` (`xo vm get web-01`) — the workaround
  `--query '[?name_label==`web-01`].id` works and should be shown in
  `usage.md`.
- Client-side `--sort` and pagination (`--offset`); `--limit` today is
  server-side only and does not reduce the batch-resolution payload.
- Richer `get` detail sheets where they are thin: `template` (adds only
  description/tags/power state over list), `network` (PIF→hosts blocked on a
  missing SDK PIF service), `pbd` (adds only `device_config`), `task`
  (Target/User still raw ids), `vbd` (device shown in header and as a row).
- `ResolveContainer` 404 on pooled objects in `vm get`/`sr get` (the lists
  already disambiguate locally with `PoolID == Container`; the detail views
  should reuse that), and the redundant master-host fetch in `pool get`
  (already contained in its `HostsOfPool` fetch-all).
- Fetching only the needed kind in `vm list`/`sr list` (both host *and* pool
  batches are fetched even when all rows use one kind).
- Snapshot/backup resources, `vm clone`/`migrate`, vif/user/group/acl
  resources, `xo watch` — all Layer 3/4 of the SDK roadmap (upstream work
  first).
- Widen `vm create`/`vm update` flags (C1's template-id acceptance is *not*
  part of this — it is already fixed); a wait-deadline `--timeout` on
  `vm create` / `network create*` (the S3 pattern from pool maintenance;
  C2) — not needed for v1.0.0 while #121 is open, but reconsider when it
  lands; second use case in `usecases.md`;
  repository `CHANGELOG.md` (GoReleaser generates per-release notes today);
  commit hash in `xo version`; group-level `Long` help text; `.gitattributes`.

## Release checklist

Minimal path from here (B1, B2 and S1–S7 are all done and merged) to a
published v1.0.0:

- [x] **B1** — fix the `rest` `-d` shorthand collision; add a test that
      exercises the real root (or `os/exec` smoke test)
- [x] **B2** — add the MIT `LICENSE` file
- [x] **S1** — process-level tests: exit codes/stderr, 401, Ctrl+C,
      `--profile` e2e, golden JSON for `vm list`/`vm get`
- [x] **S2** — error translation layer (timeout / network / unmarshal) with
      the raw detail behind `--debug`; status-based not-found detection
      (merged in #49; checkbox ticked here as a fix for an oversight)
- [x] **S3** — decide + implement (or document) wait bounds: `task wait`
      default deadline, pool-maintenance `--timeout`, export/import
      `--timeout` help note; verify the `interrupted` hang against a live
      instance and file the SDK issue (merged in `cde603c`; SDK issue
      [vatesfr/xenorchestra-go-sdk#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121))
- [x] **S4** — `--json` flag (or documented `-o json`); YAML number decision;
      JSON contract subsection in `usage.md`; mutation-JSON names-vs-ids
      decision
- [x] **S5** — one pass over the renderers: booleans, `PLATFORM`, 0-size
      values, `HOST/POOL` vs `CONTAINER`, drop the constant `network TYPE`
      column, empty-value dashes, cap `vdi get` names, empty-table message
      (branch `s5-human-output`)
- [x] **S6** — README: `--insecure` scope, platforms, command overview
      (branch `s6-readme-accuracy`)
- [x] **S7** — decide: `vm reboot --hard` confirmation (stop loses its
      confirmation, power family aligned, documented); double-login fix
      deferred to v1.1 (caveat documented, tracking issue)
      (branch `s7-ux-decisions`)
- [x] **C1** — `vm create --template`: accept the composite
      `poolId-uuid` id printed by `xo template list` (done: `parseTemplateID`
      accepts bare UUID or composite, reduces to the bare uuid for
      `create_vm`; help corrected; `TestVMCreateCompositeTemplateID` +
      `TestVMCreateInvalidTemplateID`)
- [ ] **C2** — doc note: list `vm create` / `network create*` among the
      unbounded waits in `usage.md` ("Timeouts and waiting") and in their
      help (the `interrupted`-task hang itself is tracked in
      [vatesfr/xenorchestra-go-sdk#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121))
- [ ] **C3** — mirror the CI gates (`-race`, integration, functional) in the
      `test` job of `release.yml`
- [ ] **C4** — drop the "Start it with" hint from `vm create --boot` output
- [ ] **C5** — fix the "small result document" sentence in `usage.md`
      (it is wrong for `vm create --output json`, which emits the full VM)
- [ ] merge → `main`, CI fully green including the `functional` job
- [ ] hand-push the `v1.0.0` tag
- [ ] publish the draft release + release notes (headline; SDK is plain
      upstream v1.20.0, no fork disclosure)
- [ ] smoke-test `install.sh` (amd64 + arm64) and the README quick start
- [x] open tracking issue: SDK `task.Wait` / `interrupted` (the SDK
      `replace` / PR #119 item is resolved by the v1.20.0 bump) — opened as
      [vatesfr/xenorchestra-go-sdk#121](https://github.com/vatesfr/xenorchestra-go-sdk/issues/121)

Everything in [Deferred to v1.1+](#deferred-to-v11) can wait; the
[roadmap in development.md](development.md#roadmap) remains the home for
post-1.0 planning.
