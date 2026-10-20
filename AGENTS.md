# AGENTS.md

Rules for anyone — human or AI agent — changing this repository. They are
short on purpose: each one is enforced by review, and most by tests.

`xo` is a Go command-line client for Xen Orchestra with an AWS-CLI-like UX,
built on `github.com/vatesfr/xenorchestra-go-sdk/v2`. It is not a port of the
Node.js `xo-cli`.

- How to build, test, commit and release: [CONTRIBUTING.md](CONTRIBUTING.md).
- How the code is organized and why: [docs/internals/](docs/internals/architecture.md)
  ([architecture](docs/internals/architecture.md),
  [SDK v2](docs/internals/sdk-v2.md),
  [performance](docs/internals/performance.md),
  [roadmap](docs/internals/roadmap.md)).
- What users see: [docs/index.md](docs/index.md).

## Hard rules

### 1. SDK v2 is the only Xen Orchestra API layer

```text
xo commands  →  xenorchestra-go-sdk/v2  →  Xen Orchestra REST API
```

- Use `github.com/vatesfr/xenorchestra-go-sdk/v2` only. **Never use the SDK
  v1** (JSON-RPC over WebSocket: `client/`, `pkg/services/jsonrpc/`, or
  `V1Client()`), not even as a fallback for a missing v2 operation.
- **SDK first.** Before writing a command, find the SDK v2 service,
  operation, request and response types (`go doc
  github.com/vatesfr/xenorchestra-go-sdk/v2/...`, or the module source). Do
  not guess SDK APIs. Use SDK types directly unless the CLI needs a different
  shape (table rows, data combined from several calls).
- **No second REST client.** Clients are built only by `internal/cli`:
  `cli.NewClient` (the typed services) and `cli.NewHTTPClient` (the SDK's
  exported `*client.Client`, for endpoints the typed services do not wrap,
  through `client.TypedGet` & co.). Do not create an `http.Client`, and do not
  duplicate authentication, base URL or TLS handling.
- **Sanctioned raw requests.** Four files build an `http.Request` themselves
  and send it with the SDK client's `HttpClient`, because the typed SDK lacks
  the call:
  - `internal/commands/rest/rest.go` — `xo rest`, the raw escape hatch;
  - `internal/commands/vm/xva.go` — XVA/OVA export and import streaming;
  - `internal/commands/vm/update.go` — the VM `PATCH` (`VM().Update` is not
    implemented in the SDK);
  - `internal/commands/token/token.go` — the user token endpoints.

  Do not add another without a documented reason in the file header, a row
  in [Known SDK gaps](docs/internals/sdk-v2.md#known-sdk-gaps-the-cli-works-around),
  and an upstream contribution where it makes sense.
- When the SDK lacks an operation: confirm it is missing from v2, check the
  REST API, and prefer contributing it to the SDK.
- When upgrading the SDK: read its changelog, run the full suite, and update
  [docs/internals/sdk-v2.md](docs/internals/sdk-v2.md).

### 2. Context propagation

Every SDK call receives the command's context (`cmd.Context()`), so Ctrl+C
cancels the HTTP request. Never use `context.Background()` inside command
execution.

### 3. Output contract

- stdout holds **only** the requested data; errors and diagnostics go to
  stderr. `xo vm list --output json | jq` must always work.
- Errors are concise and prefixed `Error: `; translate SDK/Go errors in
  `internal/cli/errors.go` and keep the raw error behind `--debug`.
- Exit status is `0` on success and `1` on any error.
- `--output table|json|yaml|text` and `--query` (JMESPath) on every command
  that returns data. Queries run on structured data, never on rendered
  tables. The JSON/YAML shape is public API from v1.0.0: changing it is a
  breaking change.
- Human views follow the [`list` vs `get` convention](docs/internals/architecture.md#list-vs-get-the-detail-view-convention):
  names resolved at a constant request cost, machine output untouched.

### 4. Destructive operations need `--yes`

Destructive commands (delete, overwrite, abort, rolling reboot, emergency
shutdown, …) ask for confirmation, accept `--yes` and `$XOA_YES`, and fail —
never hang — when stdin is not a terminal. Reversible power actions
(`start`, `stop`, `reboot`, …) do not prompt.

### 5. Never print secrets

Never print or log tokens, passwords or `Authorization`/cookie headers, and
never put them in error messages. Token ids are secrets: mask them unless the
user asks (`--no-secret`). Never commit credentials; tests use fakes, and
integration credentials come from `XOA_TEST_URL` / `XOA_TEST_TOKEN`. Do not
weaken TLS for convenience.

### 6. Tests are hermetic

- `go test ./...`, lint and build never need a live Xen Orchestra: use
  `httptest` servers and deterministic fixtures.
- Every test package has a `main_test.go` calling `testenv.Main(m)`
  (`internal/testenv`); tests needing a fake server call
  `testenv.Isolate(t, url)`. CI runs the suite with hostile `XOA_*` values.
- Integration tests use the `integration` build tag and report **skipped**,
  never passed, when not configured.
- A change to the exit codes, stdout/stderr split or JSON shape updates the
  process tests in `cmd/xo/process_test.go` (golden files: `-update`).

### 7. Help conventions

- Every command has `Short` and `Long` help.
- Examples go in cobra's `Example` field, never in `Long`; every example line
  must be a valid command (`internal/commands/examples_test.go`).
- After changing a command, flag or help text, regenerate the reference with
  `mise run docs` (`go run ./tools/gendocs -markdown docs/reference/cli`) and
  commit it. Never edit `docs/reference/cli/` by hand.
- Update the hand-written docs (`docs/reference/commands.md`, …) when a
  behavior they describe changes.

### 8. Never push to or merge into `main`

Work on a feature branch, push it, open or update a pull request, and stop:
merging is the maintainer's decision. Do not push to `main`, do not run
`gh pr merge` (or an equivalent) for a PR targeting `main`, and do not
force-push, rebase or rewrite `main`. "Finished" means a green pull request,
not a merged commit. Use [Conventional Commits](https://www.conventionalcommits.org/):
the type decides the next version (see
[CONTRIBUTING.md](CONTRIBUTING.md#commits-and-versioning)).

## Package layout

```text
cmd/xo/              main entrypoint; process-level tests + golden files (testdata/)
internal/
  cli/               client construction (NewClient, LoadClient, NewHTTPClient),
                     global flags, error translation, name-resolver factory,
                     shared helpers (Confirm, ParseID)
  commands/          one package per command group:
                     configure, host, network, pbd, pool, rest, sr, task,
                     template, token, vbd, vdi, vm (+ version.go, commands.go);
                     tags/ builds the shared '<resource> tag' group
  config/            profiles, XOA_* environment overrides, config file
  output/            table/json/yaml/text rendering, JMESPath (--query),
                     detail-sheet helpers
  resolve/           reference → name resolution, single and batch (anti-N+1)
  taskwait/          the shared --wait poll loop (also used by task wait)
  testenv/           test isolation from the developer's XOA_* settings
tools/gendocs/       generates docs/reference/cli (Markdown) and man pages
ci/                  xo-api-sim cookie-auth patch, perf counting proxy
```

There is no separate query package: JMESPath lives in `internal/output`.
Do not copy SDK source into the repository, and do not add a dependency for
something trivial.

## Design guidance

- Resource-oriented commands: `xo <resource> <operation>`, with explicit
  domain verbs (`xo vm start`), not generic action flags.
- Implement what the SDK and the REST API actually support, not what would
  fit the pattern.
- Commands stay thin: resolve flags, call the SDK, hand the result to
  `internal/output`. Add a wrapper or an interface only when it adds CLI
  behavior or a concrete testing benefit.
- Small, focused changes; follow the conventions of the neighboring commands.

## Definition of done

A change is complete when:

1. The SDK v2 operation it uses has been identified; no v1 code and no new
   HTTP client were introduced.
2. `--help` is useful: `Long` text, examples in `Example`.
3. Human output works, and machine output (`json`/`yaml`/`text`, `--query`)
   where applicable; stdout/stderr and exit codes follow the contract.
4. Errors are understandable without knowing Go internals.
5. Destructive operations confirm and accept `--yes`.
6. Tests cover the important behavior, including errors and confirmation,
   and stay hermetic.
7. `mise run lint` and `mise run test` pass (`gofmt`, `go vet`,
   golangci-lint, `go test ./...`).
8. The command reference is regenerated, and the user docs, internals docs
   and [CHANGELOG.md](CHANGELOG.md) are updated where relevant.
9. The work is on a feature branch with a pull request — not on `main`.
