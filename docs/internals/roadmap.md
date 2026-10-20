# Roadmap

What is not built yet, organized in four layers. The deeper a layer, the more
it depends on SDK work (see the
[architecture rules](../../AGENTS.md): nothing bypasses `xenorchestra-go-sdk/v2`).
What has shipped is in the [changelog](../../CHANGELOG.md).

- [Layer 1 — already in the SDK, not exposed by the CLI yet](#layer-1--already-in-the-sdk-not-exposed-by-the-cli-yet)
- [Layer 2 — complete SDK services, no CLI resource yet](#layer-2--complete-sdk-services-no-cli-resource-yet)
- [Layer 3 — in the REST API, not in the SDK yet](#layer-3--in-the-rest-api-not-in-the-sdk-yet)
- [Layer 4 — not in the SDK at all](#layer-4--not-in-the-sdk-at-all)
- [CLI-side polish (no SDK dependency)](#cli-side-polish-no-sdk-dependency)
- [SDK bug to verify upstream](#sdk-bug-to-verify-upstream)
- [SDK bugs confirmed & fixed upstream](#sdk-bugs-confirmed--fixed-upstream)

## Layer 1 — already in the SDK, not exposed by the CLI yet

Every item below is a command (or a small set of commands) on top of an
existing, implemented SDK service method — the same pattern as the current
commands. No SDK work is required.

- `xo template tag add|remove` — `Template().AddTag` / `RemoveTag` (since
  `v1.21.0`). Template ids are strings (the composite `<pool>-<uuid>` id of
  default templates), so `internal/commands/tags` needs to accept a string id
  first.

Each item added here reduces the need for `xo rest`.

## Layer 2 — complete SDK services, no CLI resource yet

Full resources following the usual `list / get / …` shape, built on an
implemented SDK service (verified in `v1.21.0`):

- — (empty: every SDK v1.21.0 service now has a matching CLI resource;
  `xo pbd` was the last one to be covered)

## Layer 3 — in the REST API, not in the SDK yet

These resources exist in the Xen Orchestra REST API but have no typed SDK
service. They require an upstream contribution (or the exported
`*client.Client` in the meantime, per the SDK-first rule; see
[SDK v2](sdk-v2.md#two-entry-points)). None of them can
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

## Layer 4 — not in the SDK at all

- `xo watch` (live events): `GET /events` is a **SSE** stream and the SDK has
  no SSE support. Per the architecture rules this goes through a SDK
  contribution, not an in-CLI `net/http` stream.

## CLI-side polish (no SDK dependency)

- Widen `xo vm create` beyond `--memory`/`--boot` (affinity, autoPoweron,
  clone, cloudConfig, install, extra VDIs/VIFs — all in `CreateVMParams`)
- Widen `xo vm update` beyond name/description/tags

## SDK bug to verify upstream

`VM().Restart()` POSTs to `/vms/<id>/actions/restart`, which does not appear
in the REST API OpenAPI spec (the reboot actions are `clean_reboot` /
`hard_reboot`). The CLI is not affected (`xo vm reboot` uses the correct
actions), but the SDK method should be checked against a live instance and
fixed upstream if broken.

## SDK bugs confirmed & fixed upstream

- **`payloads.VBD.Position` typed as `StringifiedInt`** (broke every
  `VBD().Get` / `GetAll` against real instances:
  `failed to unmarshal response strconv.Atoi: parsing "xvdb": invalid syntax`).
  The REST API exposes the XAPI `userdevice` under the `position` key as a
  string whose content is data dependent (a numeric index on some stacks, a
  device name such as `xvdb` / `cd0` on others); the SDK's own v1 client
  already types it as `string`. Fixed upstream as a `string` (PR
  [vatesfr/xenorchestra-go-sdk#119](https://github.com/vatesfr/xenorchestra-go-sdk/pull/119),
  regression tests added in the SDK) and released in `v1.20.0`, which
  `go.mod` now requires (see the pin history in
  [SDK v2 → Module layout](sdk-v2.md#module-layout)).

## Known follow-ups

- A single shared SDK client per invocation, so a username + password
  profile logs in once instead of twice (see
  [Token or password?](../getting-started/authentication.md#token-or-password)).
