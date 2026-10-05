# Road to v1.0.0

Status snapshot and the minimal checklist for releasing `xo` v1.0.0.

> **Snapshot**: `main` at `86807bf` (2026-10-05). B1, B2, S1, S2, S4 and S5
> are done (merged); S3 and S7 and the release-day steps remain. S6 is
> implemented on branch `s6-readme-accuracy`. The rest of this page is based
> on the original read-only audit of the full codebase (every command file,
> the output layer, the test suite, the docs and the release pipeline), with
> the built binary exercised directly to confirm the key findings. As items
> are fixed, cross them off in the [release checklist](#release-checklist)
> and update the snapshot line.

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
  `completion`. Every SDK v1.19.0 service method that maps to a single command
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
blockers are resolved (B1, B2), as are S1, S2, S4 and S5; the remaining
"should fix" items (S3 and S6–S7) and the release-day steps still need to be
done by hand.

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
   constant anti-N+1 cost; first-class vdi/vbd/pbd; `rest` escape hatch) and
   an explicit **disclosure that the SDK is a pinned fork** pending upstream
   PR vatesfr/xenorchestra-go-sdk#119 (`VBD.Position` fix — without it,
   `vbd list`/`get` cannot unmarshal real responses).
5. Smoke-test `install.sh` against the published release on Linux amd64 and
   arm64, then run the README quick start verbatim.

## Tracked follow-ups (not release-blocking)

- **SDK fork pin.** `go.mod` replaces `github.com/vatesfr/xenorchestra-go-sdk`
  with `littlejo/xenorchestra-go-sdk` at commit `35834c1` (PR #119). Verified
  at snapshot time: upstream's latest release is still v1.19.0 and PR #119 is
  still open, so the pin is load-bearing. The pseudo-version + `go.sum` make
  builds reproducible, but (a) disclose it in the release notes (see
  release-day step 4), (b) open a tracking issue to drop the `replace` and
  bump the require as soon as an upstream release includes PR #119
  (`development.md` already documents the procedure), and (c) watch for
  fork-rot (it is a personal fork).
- **SDK `task.Wait` and `interrupted`.** File upstream: the SDK's wait loop
  only treats `success`/`failure` as terminal; the CLI works around it in
  `internal/taskwait`. Relevant to the pool-maintenance hang risk in S3.
- **Three duplicated request builders** on top of the SDK HTTP client
  (`rest.doRestRequest`, `vm/xva.xvRequest`, `token.doTokensRequest`), each
  re-implementing URL joining, cookie attachment and error formatting —
  consolidate into one helper to remove the drift surface.

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
- Widen `vm create`/`vm update` flags; second use case in `usecases.md`;
  repository `CHANGELOG.md` (GoReleaser generates per-release notes today);
  commit hash in `xo version`; group-level `Long` help text; `.gitattributes`.

## Release checklist

Minimal path from here (B1, B2, S1, S2, S4 and S5 are done) to a published
v1.0.0:

- [x] **B1** — fix the `rest` `-d` shorthand collision; add a test that
      exercises the real root (or `os/exec` smoke test)
- [x] **B2** — add the MIT `LICENSE` file
- [x] **S1** — process-level tests: exit codes/stderr, 401, Ctrl+C,
      `--profile` e2e, golden JSON for `vm list`/`vm get`
- [ ] **S2** — error translation layer (timeout / network / unmarshal) with
      the raw detail behind `--debug`; status-based not-found detection
- [ ] **S3** — decide + implement (or document) wait bounds: `task wait`
      default deadline, pool-maintenance `--timeout`, export/import
      `--timeout` help note; verify the `interrupted` hang against a live
      instance and file the SDK issue
- [x] **S4** — `--json` flag (or documented `-o json`); YAML number decision;
      JSON contract subsection in `usage.md`; mutation-JSON names-vs-ids
      decision
- [x] **S5** — one pass over the renderers: booleans, `PLATFORM`, 0-size
      values, `HOST/POOL` vs `CONTAINER`, drop the constant `network TYPE`
      column, empty-value dashes, cap `vdi get` names, empty-table message
      (branch `s5-human-output`)
- [x] **S6** — README: `--insecure` scope, platforms, command overview
      (branch `s6-readme-accuracy`)
- [ ] **S7** — decide: `vm reboot --hard` confirmation; double-login fix or
      v1.1
- [ ] merge → `main`, CI fully green including the `functional` job
- [ ] hand-push the `v1.0.0` tag
- [ ] publish the draft release + release notes (headline + fork disclosure)
- [ ] smoke-test `install.sh` (amd64 + arm64) and the README quick start
- [ ] open tracking issues: drop the SDK `replace` when upstream ships PR
      #119; SDK `task.Wait` / `interrupted`

Everything in [Deferred to v1.1+](#deferred-to-v11) can wait; the
[roadmap in development.md](development.md#roadmap) remains the home for
post-1.0 planning.
