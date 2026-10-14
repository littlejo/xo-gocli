# AGENTS.md

## Project

This repository contains a new Go command-line client for Xen Orchestra.

The project is a **new `xo-cli` implementation written in Go**, with a user experience inspired by the AWS CLI.

The CLI must be built on top of:

```text
github.com/vatesfr/xenorchestra-go-sdk/v2
```

**Only the SDK v2 API must be used.**

The legacy SDK v1 API must never be used by this project.

---

# Core principles

## 1. SDK v2 is the only Xen Orchestra API layer

The CLI must use:

```go
github.com/vatesfr/xenorchestra-go-sdk/v2
```

The SDK v2 client uses the Xen Orchestra REST API.

Do not implement a second REST client inside this repository.

Do not duplicate HTTP handling already provided by the SDK.

Do not create custom REST requests when an equivalent SDK v2 operation exists.

The dependency hierarchy must remain:

```text
CLI commands
      ↓
xo-gocli domain/UX layer
      ↓
xenorchestra-go-sdk/v2
      ↓
Xen Orchestra REST API
```

Not:

```text
CLI commands
      ↓
custom REST client
      ↓
Xen Orchestra REST API
```

---

## 2. Never use SDK v1

The repository `github.com/vatesfr/xenorchestra-go-sdk` contains both v1 and v2 APIs.

The existing SDK documentation describes:

* v1: JSON-RPC API
* v2: REST API

This project must use **v2 exclusively**.

Do not import:

```go
github.com/vatesfr/xenorchestra-go-sdk
```

if doing so would result in using the legacy v1 client.

Prefer explicit v2 imports and APIs.

Before implementing functionality, inspect the SDK v2 package and documentation to determine whether the required operation already exists.

---

# SDK-first development

Before implementing a new CLI command:

1. Inspect `xenorchestra-go-sdk/v2`.
2. Find the corresponding service.
3. Find the existing SDK operation.
4. Understand its request and response types.
5. Build the CLI command on top of it.
6. Only if the SDK does not expose the required operation, consider how the gap should be handled.

Do not immediately implement a custom HTTP request because the SDK API is unfamiliar.

The SDK is the source of truth for how this CLI communicates with Xen Orchestra.

---

# CLI architecture

The CLI should add a UX layer on top of the SDK.

Conceptually:

```text
                 ┌─────────────────────┐
                 │       xo CLI        │
                 │                     │
                 │ commands            │
                 │ output              │
                 │ query               │
                 │ config              │
                 │ UX                  │
                 └──────────┬──────────┘
                            │
                            ▼
                 ┌─────────────────────┐
                 │ xenorchestra-go-sdk │
                 │        /v2          │
                 └──────────┬──────────┘
                            │
                            ▼
                 ┌─────────────────────┐
                 │ Xen Orchestra REST  │
                 │        API          │
                 └─────────────────────┘
```

The CLI should therefore focus on:

* command structure
* discoverability
* output formatting
* filtering/querying
* configuration
* authentication UX
* error presentation
* scripting
* shell completion

The SDK should handle:

* HTTP
* REST endpoints
* request/response types
* authentication mechanisms supported by the SDK
* API-specific behavior
* Xen Orchestra domain models

---

# Do not wrap the SDK unnecessarily

Avoid creating one-to-one wrappers such as:

```go
func (c *Client) ListVMs(...) {
    return c.sdk.VMs().List(...)
}
```

when the wrapper adds no value.

A wrapper is justified when it adds CLI-specific behavior, such as:

* combining multiple SDK calls
* transforming data for presentation
* implementing CLI-specific filtering
* resolving command arguments
* composing several domain operations
* hiding complexity that is genuinely specific to the CLI

Prefer direct use of the SDK when possible.

---

# SDK versioning

The CLI must explicitly depend on the v2 SDK.

The dependency should be visible in `go.mod`.

Do not depend on an unversioned or floating source checkout.

When upgrading the SDK:

1. inspect the changelog/release
2. run the full test suite
3. inspect API changes
4. verify CLI behavior
5. update the CLI only where required
6. update the "SDK v2: what we build on" section in `docs/development.md`
   so it reflects the new module layout, entry points, request model and
   known gaps

Avoid vendoring the SDK unless there is a specific reason.

---

# Handling missing SDK functionality

The SDK v2 is currently evolving.

If the CLI needs an operation that does not exist in v2:

**Do not silently fall back to v1.**

Instead:

1. Confirm that the operation is genuinely missing from v2.
2. Check whether the Xen Orchestra REST API exposes the operation.
3. Determine whether the SDK should expose the operation.
4. Prefer contributing the missing operation to the SDK when appropriate.
5. Keep the CLI implementation independent from the legacy v1 API.

The desired architecture is:

```text
Xen Orchestra REST API
        ↓
xenorchestra-go-sdk/v2
        ↓
xo CLI
```

not:

```text
Xen Orchestra REST API
        ↓
xo CLI
        ↓
v1 compatibility hack
```

---

# Raw REST access

The CLI may provide a low-level REST escape hatch if useful.

However, this must **not become a second REST client implementation**.

The preferred approach is:

```text
xo rest ...
      ↓
SDK v2 HTTP facilities
```

If SDK v2 does not expose a generic REST operation required by this feature, stop and evaluate whether that capability belongs in the SDK instead.

Do not bypass the SDK casually.

The raw REST feature must never import or use the SDK v1 client.

---

# AWS CLI-inspired UX

The CLI should feel familiar to users of AWS CLI.

Prefer:

```bash
xo vm list
xo vm get <id>
xo vm start <id>
xo vm stop <id>

xo host list
xo pool list
xo sr list
xo network list
xo task list
```

over API-oriented commands such as:

```bash
xo GET /v0/vms
```

The exact command hierarchy should follow Xen Orchestra concepts and the capabilities exposed by SDK v2.

---

# Resource-oriented design

Use a consistent resource model:

```text
xo <resource> <operation>
```

Examples:

```bash
xo vm list
xo vm get <id>

xo host list
xo host get <id>

xo pool list
xo pool get <id>

xo sr list
xo sr get <id>
```

Domain-specific operations may be explicit:

```bash
xo vm start <id>
xo vm stop <id>
xo vm reboot <id>
xo vm snapshot <id>
```

Avoid generic action flags such as:

```bash
xo vm --action=start
```

when an explicit command is clearer.

---

# Output

Output is a first-class CLI feature.

Support:

```text
--output table
--output json
--output yaml
--output text
```

The default should be optimized for humans.

Machine-readable output must contain only the requested data.

For example:

```bash
xo vm list --output json
```

must produce valid JSON on stdout.

Errors and diagnostics belong on stderr.

This must work:

```bash
xo vm list --output json | jq '.[].name_label'
```

---

# Query support

Provide an AWS CLI-like query mechanism where practical:

```bash
xo vm list --query '[].name_label'
```

and:

```bash
xo vm list --query '[?power_state==`Running`].name_label'
```

Prefer a mature existing Go implementation rather than inventing a query language.

The processing pipeline should be:

```text
SDK response
     ↓
structured data
     ↓
query/filter
     ↓
output formatter
```

Never parse rendered tables to implement queries.

---

# Configuration

Support multiple Xen Orchestra instances through profiles.

The UX should resemble AWS CLI:

```bash
xo configure
xo configure --profile lab
xo configure --profile production
```

Commands can select a profile:

```bash
xo vm list --profile lab
xo vm list --profile production
```

and preferably:

```bash
export XOA_PROFILE=production
```

Configuration should contain only CLI-level configuration.

Do not duplicate SDK configuration concepts unnecessarily.

If SDK v2 already provides a configuration or client-construction mechanism, use it.

---

# Authentication

Authentication must use the mechanisms supported by SDK v2.

Do not reimplement authentication at the CLI level if the SDK already handles it.

The CLI may provide UX around credentials:

```bash
xo configure
```

but the actual authentication mechanism should remain compatible with the SDK.

Never:

* print tokens
* log Authorization headers
* expose credentials in errors
* commit credentials
* include real credentials in tests

---

# Command implementation

Commands should depend on the SDK v2 client through explicit dependencies.

Avoid global clients.

Prefer dependency injection:

```go
type Dependencies struct {
    XO     *xo.Client
    Config Config
    Output Output
}
```

or an equivalent simple structure.

The exact types depend on the SDK API.

Do not invent interfaces for every SDK type.

Only introduce an interface when it provides a concrete testing or architectural benefit.

---

# SDK types vs CLI types

Prefer using SDK v2 response types directly when the CLI only needs to display them.

Do not automatically duplicate every SDK model:

```go
type VM struct {
    ...
}
```

if the SDK already exposes an appropriate VM type.

Create CLI-specific types only when the CLI needs a representation that is meaningfully different.

Examples where a CLI-specific type may make sense:

* flattened table rows
* combined data from several API calls
* human-readable status
* aggregated output

Keep such transformations close to the output/UX layer.

---

# Resource commands

A resource should ideally expose a predictable command set:

```text
list
get
create
update
delete
```

when supported by Xen Orchestra and SDK v2.

Additional operations should reflect domain behavior:

```text
start
stop
reboot
snapshot
migrate
...
```

Do not implement commands merely because they would fit the pattern.

The SDK and actual Xen Orchestra API capabilities determine what is valid.

---

# Vertical implementation strategy

Do not attempt to expose the entire SDK immediately.

Implement one resource end-to-end.

Recommended first slice:

```text
xo configure
      ↓
SDK v2 client
      ↓
xo vm list
      ↓
table/json/yaml
      ↓
--query
```

Then:

```text
xo vm get <id>
xo vm start <id>
xo vm stop <id>
```

Once this UX is proven, add more resources.

This allows the project to validate its architecture before generating a large amount of code.

---

# SDK exploration

When an agent is asked to implement a new command, it should inspect the SDK first.

Useful investigation steps may include:

```bash
go doc github.com/vatesfr/xenorchestra-go-sdk/v2
go doc github.com/vatesfr/xenorchestra-go-sdk/v2/...
```

and inspecting the module source when necessary.

The agent should identify:

```text
service
operation
request type
response type
error behavior
```

before writing the command.

Do not guess SDK APIs.

---

# Error handling

Translate SDK/API errors into concise CLI errors.

Prefer:

```text
Error: VM "web-01" not found
```

over exposing Go internals.

However, preserve useful API information when available.

Debug mode may expose more details.

Normal users should not need to understand:

```text
json.Unmarshal
http.Client
reflect
```

to understand why an operation failed.

---

# Destructive operations

Commands such as:

```bash
xo vm delete <id>
xo vm stop <id>
```

must be explicit.

Destructive operations may require confirmation.

Provide a non-interactive option:

```bash
xo vm delete <id> --yes
```

Automation must never hang waiting for input.

---

# Context

Every SDK operation must receive the command context where the SDK API supports it.

Cancellation must propagate from:

```text
Ctrl+C
   ↓
CLI command
   ↓
SDK
   ↓
HTTP request
```

Do not use `context.Background()` deep inside command execution when a command context is available.

---

# Testing

Test the CLI independently from Xen Orchestra where possible.

Prioritize:

### Command tests

Test:

* argument parsing
* flags
* command behavior
* output
* errors
* confirmation handling

### SDK integration boundaries

Mock or fake the smallest useful abstraction.

Do not reproduce the entire SDK in mocks.

### Integration tests

Where practical, run against a real Xen Orchestra instance.

Never hard-code credentials.

Integration tests should be explicitly enabled/configured.

### Use case tests (each use case is tested in CI)

Every use case documented in `docs/usecases.md` must be pinned by an
integration test in `internal/commands/usecases_integration_test.go`. The test
drives the real command tree end to end with typed `xo` commands (no `xo rest`),
asserts the machine output at each step, and verifies the final state.

These tests run in CI without a live instance or any credential: the `functional`
job of `.github/workflows/ci.yml` starts the xo-api-sim REST simulator, exports
`XOA_TEST_URL` / `XOA_TEST_TOKEN`, and runs `go test -tags=integration ./...`, so
each documented use case is exercised end to end with real HTTP requests.

Rules:

* A use case is only *complete* when every step that is a typed `xo` command is
  covered by the test (see the definition of a complete use case in
  `docs/usecases.md`).
* Steps that cannot be automated are excluded **and** the reason is stated in a
  comment in the test — for example guest-internal work (`mkfs`/`mount`), the
  VM -> template conversion (no REST endpoint, done in the web UI), or the OS
  actually booting (the test proves the power-state transitions, not the guest).
* When a new use case is added to `docs/usecases.md`, its integration test must
  be added in the same change and must be green in CI before the change is done.
* Without `XOA_TEST_URL` / `XOA_TEST_TOKEN` the use case tests are **skipped**,
  never treated as passed — the same contract as every other integration test.

---

# Dependency rules

The CLI should have a small dependency set.

The Xen Orchestra SDK is a core dependency:

```text
github.com/vatesfr/xenorchestra-go-sdk/v2
```

Do not introduce another Xen Orchestra client.

Do not introduce a second REST implementation.

For CLI functionality, prefer mature dependencies for:

* CLI parsing
* query expressions
* YAML
* terminal/table rendering

Avoid dependencies for trivial functionality.

---

# Project structure

A reasonable initial structure is:

```text
cmd/
    xo/

internal/
    cli/
    commands/
    config/
    output/
    query/
```

The SDK remains external:

```text
github.com/vatesfr/xenorchestra-go-sdk/v2
```

Do not copy SDK source into this repository.

The exact package layout can evolve as the codebase grows.

---

# Vibe coding rules

This repository is intentionally designed for AI-assisted development.

Agents should:

* inspect existing code first
* inspect SDK v2 before implementing XO functionality
* make small changes
* run tests after changes
* run `gofmt`
* run static analysis
* verify CLI behavior
* keep changes focused
* prefer simple code
* reuse SDK functionality

Agents must not:

* use SDK v1
* implement a parallel REST client
* copy the Node.js xo-cli architecture
* duplicate SDK models without a reason
* generate the entire CLI in one change
* invent SDK APIs
* add speculative abstractions
* silently change established CLI conventions
* weaken TLS/security for convenience
* push to `main` or merge into `main` (see [Repository workflow](#repository-workflow-never-touch-main))

---

# Definition of done

A CLI feature is complete when:

1. The corresponding SDK v2 functionality has been identified.
2. The command uses SDK v2.
3. No v1 API is used.
4. No duplicate REST client was introduced.
5. `--help` is useful.
6. Human-readable output works.
7. Machine-readable output works where applicable.
8. Errors are understandable.
9. Tests cover important behavior.
10. `gofmt` has been run.
11. Static analysis has been considered.
12. Documentation/examples are updated when appropriate.

---

# Architectural rule

The most important architectural rule in this repository is:

```text
                    UX
                     │
                     ▼
                  xo-gocli
                     │
                     ▼
       xenorchestra-go-sdk/v2
                     │
                     ▼
          Xen Orchestra REST API
```

There must be **one Xen Orchestra client implementation**:

```text
xenorchestra-go-sdk/v2
```

The CLI exists to make that SDK and Xen Orchestra API pleasant for humans and automation.

Do not bypass this architecture without a deliberate, documented reason.

---

# Design philosophy

Prefer:

```text
simple
explicit
discoverable
scriptable
```

over:

```text
clever
abstract
magical
```

Prefer:

```text
one useful vertical slice
```

over:

```text
complete API coverage
```

Prefer:

```text
SDK v2 capability
```

over:

```text
custom implementation
```

The goal is not to reproduce the existing `xo-cli`.

The goal is to create a **modern Go CLI for Xen Orchestra**, with an AWS CLI-like experience, while keeping `xenorchestra-go-sdk/v2` as the single API boundary.


# Repository workflow: never touch `main`

Agents must **never push to `main`** and must **never merge into `main`**
(no direct push, no PR merge, no rebase of `main`). Merging into `main` is
always the maintainer's decision.

The workflow is:

```text
feature branch
      ↓
push the feature branch
      ↓
open/update a pull request
      ↓
stop: the maintainer reviews and merges
```

So:

* `git push` may only ever target a feature branch, never `main`.
* Do not run `gh pr merge` (or any equivalent) for a PR targeting `main`;
  open the PR and report its URL instead.
* Do not force-push, rebase or rewrite `main`.

This applies even when a task is described as "finishing" or "completing"
work: the finished state is a green pull request, not a merged commit.


# Xen Orchestra integration environment

A Xen Orchestra instance is **not required for normal development**.

Agents must be able to implement and test most functionality without access
to a live Xen Orchestra instance.

Use:

- unit tests
- SDK mocks/fakes where appropriate
- `httptest.Server`
- deterministic fixtures

for normal development.

A real Xen Orchestra instance is required for integration testing of
functionality that depends on actual Xen Orchestra behavior.

When a live XO instance is available, integration tests should use it rather
than assuming that mocked SDK behavior is sufficient.

Integration credentials must never be committed.

The integration environment should be configured through environment
variables or an explicit configuration file that is excluded from git.

For example:

    XOA_TEST_URL
    XOA_TEST_TOKEN

Do not require a live XO instance as a prerequisite for:

- `go test ./...`
- formatting
- linting
- building the CLI

Integration tests should be explicitly enabled or detected from their
environment.

For example:

    go test ./... -tags=integration

or an equivalent mechanism.

If an integration test cannot run because no Xen Orchestra instance is
configured, this must be clearly reported rather than silently treated as a
successful integration test.
