# Architecture

How the code is organized and the conventions the human views follow. For
the rules a change must respect, see [AGENTS.md](../../AGENTS.md); for the
build and test workflow, see [CONTRIBUTING.md](../../CONTRIBUTING.md).

- [Layers](#layers)
- [Repository layout](#repository-layout)
- [`list` vs `get` (the detail-view convention)](#list-vs-get-the-detail-view-convention)
- [Invariants](#invariants)

## Layers

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
contributed upstream where appropriate. What the SDK provides, and where the
CLI works around it, is described in [SDK v2](sdk-v2.md).

## Repository layout

```text
cmd/xo/                  main entrypoint; process-level tests (process_test.go)
                           and their golden files (testdata/)
internal/
  cli/                   SDK client construction (NewClient, NewHTTPClient),
                           global flags, error translation and hints,
                           name-resolver factory (NewResolver)
  commands/              one package per resource group (vm, host, pool, …)
    configure/           profile management
    rest/                the raw REST escape hatch
  config/                profiles, environment overrides, config file
  output/                table/json/yaml/text rendering, JMESPath queries,
                           detail-sheet helpers (DetailField, OrDash, JoinNames)
  resolve/               reference→name resolution (single + batch), the
                           shared anti-N+1 helper behind every `get` detail view
  taskwait/              shared --wait poll loop (reused by task wait)
  testenv/               test isolation from the developer's XOA_* settings
  testutil/              shared test helpers: root command, runner, fake-server auth
tools/gendocs/           generates docs/reference/cli and the man pages
ci/                      xo-api-sim cookie-auth patch, perf counting proxy
```

## `list` vs `get` (the detail-view convention)

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

The relationships are resolved by the shared [`internal/resolve`](../../internal/resolve/resolve.go)
helper (built on the SDK v2 typed services, plus the SDK REST client for the
endpoints they don't wrap yet, such as `vm-templates`). See the
[command guide](../reference/commands.md) for each resource's detail view and
example output, and [Performance](performance.md) for how to verify the cost
model.

## Invariants

These hold across the code base; the tests pin most of them.

- **One API boundary.** Every request goes through the SDK v2: the typed
  services (`cli.NewClient`) or the SDK's exported REST client
  (`cli.NewHTTPClient`). See [SDK v2](sdk-v2.md).
- **Output contract.** stdout holds only the requested data; errors go to
  stderr with an `Error: ` prefix; the exit status is `0` or `1`
  ([Exit codes](../reference/exit-codes.md)). The JSON and YAML shape is the
  public API ([The JSON and YAML contract](../reference/output-and-query.md#the-json-and-yaml-contract)).
- **Queries run on structured data**, never on rendered tables: SDK response →
  structured data → `--query` → formatter.
- **Constant request cost** for the human views, and exactly one request for
  machine output ([Performance](performance.md#invariants-to-preserve)).
- **Cancellation propagates**: Ctrl+C cancels the command context, which the
  SDK passes down to the HTTP request.
- **Destructive commands confirm**, accept `--yes` / `$XOA_YES`, and fail
  instead of prompting when stdin is not a terminal; they all go through
  `cli.Confirm`.
