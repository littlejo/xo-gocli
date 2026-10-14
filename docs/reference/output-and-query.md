# Output and querying

How `xo` renders results, how to filter them with `--query`, and what the
machine-readable formats guarantee to scripts.

- [Querying with `--query`](#querying-with---query)
- [Choosing the default format](#choosing-the-default-format)
- [The JSON and YAML contract](#the-json-and-yaml-contract)
- [Mutation and action output](#mutation-and-action-output)
- [Human view conventions](#human-view-conventions)

## Querying with `--query`

The pipeline is always: **SDK response → structured data → query → formatter**.
Queries operate on the structured data, never on rendered tables.

```sh
xo vm list --output json                      # full objects as JSON
xo vm list --query '[].name_label'            # one value per line
xo vm list --query '[?power_state==`Running`].name_label'
xo vm list --query 'length(@)'                # count
```

Backtick literals work as in the AWS CLI. A backtick literal holds JSON, so
it keeps its type: `` `true` `` is a boolean, `` `2` `` a number,
`` `"Running"` `` a string. Text that is not valid JSON, such as
`` `Running` ``, is read as a string, as the AWS CLI does. Single-quoted
strings (`'Running'`) are the plain JMESPath form and always work.

```sh
xo pool list --query '[?HA_enabled==`true`].name_label'
xo vm list --query '[?CPUs.number > `2`].name_label'
```

Every `list` command also takes `--limit N` (0, the default, means no limit).
The limit is applied by the server, before `--query`:
``xo vm list --limit 10 --query '[?power_state==`Running`]'`` filters the
first 10 VMs, not all of them.

## Choosing the default format

`--output` applies to one invocation. For a recurring default, set
`XOA_DEFAULT_OUTPUT` for the shell or script (`XOA_DEFAULT_OUTPUT=json xo vm
list`), or store `output: json` on the profile in the configuration file.
Resolution order: `--output` > `--json` > `$XOA_DEFAULT_OUTPUT` > the
profile's `output` > `table`. The `--json` flag is a boolean shortcut for
`--output json` (AWS CLI familiarity); an explicit `--output` still wins
when both are given.

```sh
export XOA_DEFAULT_OUTPUT=json        # everything from this shell is JSON
xo vm list                            # JSON, no flag needed
xo vm list --output table             # back to the human view for once
```

## The JSON and YAML contract

`--output json` emits the **SDK struct shape**, not the API response
byte-for-byte:

- fields declared `omitempty` in the SDK payload are dropped when they hold
  an empty scalar (`""`, `0`, `false`), so two responses to the same object
  may have different field sets;
- empty collections are always present, as `[]` or `{}` (a VM without tags
  has `"tags": []`, as the Xen Orchestra API returns it), so functions such
  as `contains(tags, 'x')` or `length(tags)` work on every object;
- fields the SDK payload does not model at all are absent (a `rest get` of
  the same object can show more fields — it prints the raw API document);
- the output is stable and intentional, and this is the v1 machine interface:
  scripts should rely on field *presence* plus the documented resource shape
  (the `--query` projections in each section of the
  [command guide](commands.md)), not on exact equality
  with a captured response.

YAML notes:

- whole numbers are printed as plain integers (`size: 2147483648`); values
  that are not whole stay decimal (`usage: 1.5`);
- YAML goes through the same JSON normalization as `--query`, so a whole
  value larger than 2^53 (about 9.007e15) can lose precision — use `--output
  json` or `--query` for such fields;
- `--output json` is the recommended format for scripting: it is the most
  faithful to the data and the most tooling-friendly (`jq`).

## Mutation and action output

Mutating commands fall into two output styles under `json`/`yaml`:

- **Commands that re-read the resource** — `vm create`, `vm update`,
  `network create`, `token create` — emit the **full object**, the same shape
  as `get`: they re-fetch the freshly created/updated resource and return it
  whole.
- **Commands that do not** — every `delete`, the action verbs (`start`,
  `stop`, `reboot`, …), and `vdi create` / `vbd create` — emit a **small
  result document**:

  ```json
  { "action": "delete", "vm": "web-01" }
  ```

  The resource field carries the **name** the command resolved for the target
  (the name the user sees in `list`), and actions that start a task add a
  `task_id` (or `result`/`task` on `--wait`). The id is not repeated because
  the name is the stable handle in the CLI; for the raw id, use the resource's
  `list`/`get` with `--query '[].id'` or the `--output json` of `get`.

Format behavior:

- `table` (default): aligned columns for humans; for `get`, a detail view —
  `vm`, `vbd`, `pbd`, `vdi`, `sr`, `pool`, `network`, `host`, `template` and
  `task` `get` are all key/value detail sheets that resolve relationships to
  names (see each group in the [command guide](commands.md)). In `list` tables, the columns that
  reference another object (`MASTER`, `SR`, `VM`/`VDI`, `POOL`, `CONTAINER`)
  show the referenced object's **name** instead of its UUID, resolved in one
  batch per referenced kind (a constant cost, never one lookup per row); a
  reference to a deleted object keeps its raw UUID.
- `json` / `yaml`: the full structured data (or the `--query` projection).
  Machine-readable output is the only thing on stdout; errors go to stderr, so
  `xo vm list --output json | jq '.[].name_label'` always works.
- `text`: a compact key/value form; a list of objects becomes an auto-column
  table, a list of scalars one value per line.

## Human view conventions

The human views (table and detail sheets) share a few conventions:

- **Booleans** are `yes` / `no` everywhere (list columns like `HA`, `DEFAULT`,
  `ATTACHED`, and detail sheets).
- **`VERSION`** is the platform version (host: `version`, pool:
  `platform_version`). The detail sheets of `host` / `pool` / `template`
  label the composed platform string `Platform` when they show brand +
  version + build.
- **`CONTAINER`** names the object a resource lives in, whether it is a pool
  or a host (`vm list`, `sr list`).
- **Sizes** that are zero render as `0 B` in list tables and as `-` in detail
  sheets (a sheet is always complete: no dangling `Label:` line); a value the
  API does not provide renders as `-`.
- **An empty list is not silent**: the table header is printed followed by
  `No <resources> found.` (`xo vm list` on an empty pool). Machine output is
  unaffected and still emits the empty structure (`[]` / `{}`).
