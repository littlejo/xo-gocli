# Performance

The human views resolve references to names; this page explains how that
stays cheap, how to measure it, and the numbers to compare against.

- [Invariants to preserve](#invariants-to-preserve)
- [How to measure it (old vs new, through a counting proxy)](#how-to-measure-it-old-vs-new-through-a-counting-proxy)
- [Reference numbers (xo-api-sim, 500 VMs / 30 hosts / 2 pools)](#reference-numbers-xo-api-sim-500-vms--30-hosts--2-pools)
- [Known trade-off](#known-trade-off)

The [`list` vs `get` rules](architecture.md#list-vs-get-the-detail-view-convention)
define a **cost model**, not just a style: the human views pay
a constant number of batch requests, independent of the number of rows, and
the machine output pays nothing at all. The unit tests pin part of that (each
`list_detail_test.go` counts the requests its fake server receives and asserts
the batch cost is exactly one `GetAll`), but when touching `internal/resolve`
or a `list`/`get` renderer it is worth re-verifying the model end-to-end with
the recipe below.

## Invariants to preserve

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

## How to measure it (old vs new, through a counting proxy)

Run two builds (the base, e.g. `origin/main`, and the change) against the
xo-api-sim simulator (started as in the
[functional tests section](../../CONTRIBUTING.md#functional-tests-against-the-simulator)), fronted
by the request-counting proxy from [`ci/perf/proxy.py`](../../ci/perf/proxy.py),
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

## Reference numbers (xo-api-sim, 500 VMs / 30 hosts / 2 pools)

Measured through the counting proxy, comparing the build before `list`
tables resolved references to names with the build after; the millisecond figures are orientation
only (local loopback, tiny payloads) — the **request counts are the
invariant**:

| Invocation | Raw UUIDs (before name resolution) | Names resolved (current) |
| ---------- | ------------- | ------------ |
| `vm list` — 500 rows, table | 1 req, ~35 ms | 3 req, ~90 ms |
| `vm list --output json` | 1 req, ~44 ms | 1 req, ~44 ms (unchanged) |
| ``vm list --query '[?power_state==`Running`].name_label'`` | 1 req | 1 req (unchanged) |
| `host list` — 30 rows, table | 1 req | 2 req (+1 batch) |
| `pbd list` — 11 rows, table | 1 req | 4 req (+3 batches) |

## Known trade-off

The batch `GetAll` fetches the **whole collection** of the referenced kind
(`fields=*`), even when only a few of its objects are actually referenced —
constant in request count, but the payload grows with farm size. That is the
price of the anti-N+1 design (the same model the XO web UI uses), and it is
fine for the kinds lists reference today (hosts, pools, SRs are small
collections). If a referenced kind ever grows large (thousands of objects),
the follow-up is to restrict the batch fetch to the fields it needs
(`id`, `name_label`) rather than reverting to per-row lookups.
