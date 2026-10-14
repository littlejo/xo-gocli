# Contributing to `xo`

How to build, test and release `xo`, and how a change gets merged. The
architecture rules every change must respect are in [AGENTS.md](AGENTS.md)
(the important one: `xenorchestra-go-sdk/v2` is the only Xen Orchestra API
layer). How the code is organized is described in
[docs/internals/](docs/internals/architecture.md).

- [Toolchain](#toolchain)
- [Testing](#testing)
  - [Hermetic tests](#hermetic-tests)
  - [Integration tests](#integration-tests)
  - [Functional tests against the simulator](#functional-tests-against-the-simulator)
  - [Process-level tests (the scripting contract)](#process-level-tests-the-scripting-contract)
- [Help text and the command reference](#help-text-and-the-command-reference)
- [Commits and versioning](#commits-and-versioning)
- [Pull requests](#pull-requests)
- [CI](#ci)
- [Release process](#release-process)
- [License](#license)

## Toolchain

A [mise](https://mise.jdx.dev/) config pins the Go toolchain and
golangci-lint (`mise.toml`):

```sh
mise install          # install the pinned Go toolchain and golangci-lint
mise run build        # go build -o dist/xo ./cmd/xo
mise run test         # go test ./...
mise run lint         # golangci-lint run + go vet + gofmt check
mise run docs         # regenerate docs/reference/cli from the command tree
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
and deterministic fixtures). A live instance is never required for
`go test ./...`, formatting, linting or building.

### Hermetic tests

The tests do not depend on your own xo setup. Every test package has a
`TestMain` that calls `testenv.Main` (`internal/testenv`): it unsets every
`XOA_*` variable except the `XOA_TEST_*` integration settings, and points
`XOA_CONFIG_FILE` at an empty temporary directory. A test that needs a fake
server calls `testenv.Isolate(t, url)`.

A new test package must add the same one-line `main_test.go`:

```go
package mypkg

import (
	"testing"

	"github.com/littlejo/xo-gocli/internal/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }
```

CI runs the suite once with hostile `XOA_*` values to catch a package that
forgets.

### Integration tests

Integration tests are opt-in and only run when explicitly enabled:

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

The simulator is also the base for measuring the request cost of the human
views; see [Performance](docs/internals/performance.md).

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

## Help text and the command reference

Every command has a `Long` help text, and its examples go in cobra's
`Example` field, never inside `Long` (`internal/commands/examples_test.go`
checks both, and checks that every example line is a valid command).

The command reference in `docs/reference/cli/` is generated by
`tools/gendocs` from the cobra tree: after changing a command, a flag or a
help text, regenerate it and commit the result:

```sh
mise run docs        # or: go run ./tools/gendocs -markdown docs/reference/cli
```

`TestMarkdownReferenceIsUpToDate` fails when it is stale. Never edit
`docs/reference/cli/` by hand. The man pages are generated the same way at
release time (a GoReleaser `before` hook) and shipped under `man/` in every
archive.

The hand-written user documentation (`docs/getting-started/`,
`docs/reference/commands.md`, `docs/reference/output-and-query.md`, …)
explains what `--help` does not; update it when a behavior changes.

## Commits and versioning

Releases follow [semantic versioning](https://semver.org/) and commits follow
[Conventional Commits](https://www.conventionalcommits.org/)
(`feat: …`, `fix(vm): …`, `docs: …`, `test: …`, `ci: …`, `chore: …`).
The commit type decides the next version. On every push to `main`,
`.github/scripts/bump-version.sh` looks at the commits since the last
`vX.Y.Z` tag:

- a commit whose subject starts with `feat:` or `feat(scope):` bumps the
  **minor** version and resets the patch (`1.0.0` → `1.1.0`);
- any other change bumps the **patch** version (`1.0.0` → `1.0.1`);
- a range that only touches documentation — `docs/`, `README.md`,
  `AGENTS.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `SECURITY.md` — creates
  **no tag**: a release ships the binary and `go.mod`, nothing in the
  documentation, so doc fixes are not released on their own;
- if `HEAD` is already tagged, nothing happens.

The script never bumps the **major** version, and a `!` breaking-change
marker does not change that (`feat!:` without a scope is not even read as a
feature). A major version is tagged by hand: `v1.0.0` was. From 1.0.0 on, a
breaking change to the commands, flags, exit codes or JSON/YAML output needs
a new major version (see the [changelog](CHANGELOG.md)).

Record user-visible changes in [CHANGELOG.md](CHANGELOG.md).

## Pull requests

Work on a feature branch and open a pull request against `main`. Never push
to `main`, and do not merge your own pull request: merging is the
maintainer's decision.

```text
feature branch
      ↓
push the feature branch
      ↓
open/update a pull request
      ↓
CI green → the maintainer reviews and merges
```

Before opening it, run the checks CI runs: `mise run lint`, `mise run test`,
and `mise run docs` if a command or help text changed.

## CI

- **CI** (`.github/workflows/ci.yml`): runs on push to `main` and on every PR —
  `gofmt`, `go vet`, `golangci-lint`, unit + integration tests (including a
  `-race` pass to catch data races early, and the hostile `XOA_*` pass
  described in [Hermetic tests](#hermetic-tests)), build. A second
  `functional` job spins up the xo-api-sim REST simulator (pinned commit,
  cookie-auth patch) and runs the integration suite against it, so every push
  is tested end-to-end over real HTTP without a live instance. The workflow is
  also reusable (`workflow_call` with a `ref` input) so other workflows — in
  particular the release — can run the exact same gates on a specific ref.
  It is the merge gate.
- **OCR PR Review** (`.github/workflows/ocr-review.yml`): AI code review on
  pull requests with [OpenCodeReview](https://open-codereview.ai)
  (`alibaba/open-code-review`), pinned to the commit SHA of a release. It
  runs the `ocr` review over the PR diff (base → head), using the Vates
  LiteLLM gateway (`qwen3-coder`, OpenAI-compatible protocol), and posts the
  findings as inline review comments plus a sticky summary on the PR. It is
  **advisory, non-blocking by design** — do not add it to the
  branch-protection required checks; the merge gate stays the `CI` workflow.
  It triggers on `pull_request` (opened / synchronize / reopened) and on a
  human (MEMBER / OWNER / COLLABORATOR) comment starting with
  `/open-code-review` or `@open-code-review` to re-run a review on demand.
  One required secret: `OCR_LLM_AUTH_TOKEN` (the LLM API key, under
  *Settings → Secrets and variables → Actions*). Because the trigger is
  `pull_request` (not `pull_request_target`), repo secrets are not available
  to fork PRs, so they are skipped outright (job-level condition and
  concurrency predicate both check
  `head.repo.full_name == github.repository`) rather than burning a
  guaranteed-failing run. The raw review JSON and stderr are cat'ed into the
  workflow log; they are deliberately not uploaded as artifacts, so the LLM
  token is never persisted to a downloadable artifact.

## Release process

Normal flow, once a pull request is merged into `main`:

1. **Version** (`.github/workflows/version.yml`) runs `bump-version.sh` (see
   [Commits and versioning](#commits-and-versioning)). If a new version is
   due, it pushes the `vX.Y.Z` tag and dispatches the Release workflow
   (a tag pushed with `GITHUB_TOKEN` does not trigger workflows on its own).
2. **Release** (`.github/workflows/release.yml`) resolves the tag, runs the
   full CI gate suite on it (reusing the CI workflow against the tag ref, so
   a release is only built on a commit that passed exactly the same gates as
   CI), then builds cross-platform binaries with
   [GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`): Linux, macOS
   and Windows on amd64 and arm64, with the man pages, a checksums file, and
   `.zip` archives for Windows.
3. GoReleaser creates the GitHub release as a **draft**. A maintainer
   reviews it and publishes it.

The Release workflow can also be run manually from the Actions tab
(optionally targeting a specific tag; without one, it releases the latest
tag). To release a specific commit by hand, a maintainer tags it and pushes
the tag, which triggers the Release workflow directly:

```sh
git tag v1.0.0 && git push origin v1.0.0
```

## License

This project is licensed under the [MIT License](https://opensource.org/licenses/MIT),
the same license as the Xen Orchestra Go SDK.
