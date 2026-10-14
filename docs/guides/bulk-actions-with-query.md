# Bulk actions with `--query`

`xo` acts on one object per command, by UUID. To act on many, select their
ids with a JMESPath `--query`, check the selection, then loop over it.

```text
   1. write the filter                    xo vm list --query "[?...]"
   2. dry run: show what would change     --query "[?...].name_label"
   3. extract the ids                     --query "[?...].id" --output text
   4. loop over them                      xo vm stop/start/tag ... --wait
```

## Prerequisites

- A configured profile (see [Configuration](../getting-started/configuration.md)).
- `jq` is optional: everything below uses the built-in `--query`.
- A quick look at [Output and query](../reference/output-and-query.md).

## JMESPath in `xo`: the essentials

`--query` is applied to the JSON form of the result, so it uses the JSON
field names (`name_label`, `power_state`, `tags`, `mainIpAddress`, ...), not
the table headers. Print one object with `--output json` to see them.

| You want | Expression |
| -------- | ---------- |
| A field of every item | `[].name_label` |
| Items matching a value | `[?power_state=='Running']` |
| Same, AWS CLI style | ``[?power_state==`Running`]`` |
| Several conditions | `[?power_state=='Halted' && contains(name_label, 'web')]` |
| Items having a tag | `[?contains(tags, 'prod')]` |
| Items without a tag | `[?!contains(tags, 'prod')]` |
| Items with no tag at all | ``[?length(tags) == `0`]`` |
| A field starting with | `[?starts_with(name_label, 'ci-')]` |
| A few fields per item | `[].{id: id, name: name_label, ip: mainIpAddress}` |
| The number of items | `length([?power_state=='Running'])` |
| The first match | `[?name_label=='web-01'].id \| [0]` |
| A field whose name starts with `$` | `[?"$poolId"=='<pool-id>']` |

Quoting rules that save time:

- Wrap the whole expression in **double quotes** in the shell and use
  **single quotes** for JMESPath strings: `"[?name_label=='web-01']"`.
- Backtick literals hold JSON, as in the AWS CLI: `` `true` `` is a boolean,
  `` `2` `` a number, and text that is not JSON (`` `Running` ``) a string.
  Inside a double-quoted shell string, escape the backticks (`` \` ``) or
  use single quotes for the whole expression. A boolean field can also be
  tested directly: `[?auto_poweron]` or `[?!auto_poweron]`.
- Every object has its collections, even empty (`"tags": []`), so
  `contains(tags, 'x')` works on untagged VMs too.
- A `$` in a field name must be double-quoted for JMESPath, and escaped for
  the shell inside double quotes: `"[?\"\$poolId\"=='<pool-id>']"`.

`--limit` is applied **by the server before** `--query`: `xo vm list --limit
10 --query "[?power_state=='Running']"` filters the first 10 VMs, it does not
return 10 running VMs. Leave `--limit` out when filtering. For power state,
`xo vm list --power-state Running` filters on the server.

## Step 1 and 2: write the filter and dry-run it

Always look at the selection before acting on it:

```sh
xo vm list --query "[?contains(tags, 'staging') && power_state=='Running'].{id: id, name: name_label}"
```

## Step 3: extract the ids

```sh
xo vm list --query "[?contains(tags, 'staging') && power_state=='Running'].id" --output text
```

`--output text` prints a list of scalars one per line, ready for a shell
loop.

## Step 4: loop over them

Stop every running staging VM, one at a time, waiting for each:

```sh
set -euo pipefail
ids=$(xo vm list --query "[?contains(tags, 'staging') && power_state=='Running'].id" --output text)
for id in $ids; do
  xo vm stop "$id" --wait --output json > /dev/null && echo "stopped $id" || echo "FAILED $id" >&2
done
```

- `--wait` makes each stop finish before the next one starts, and turns a
  failed task into a non-zero exit. Without it, all the stops are requested
  at once.
- The `&& ... ||` keeps the loop going when one VM fails; drop it to stop at
  the first failure.

Start them again (`xo vm start` also has `--wait`):

```sh
ids=$(xo vm list --query "[?contains(tags, 'staging') && power_state=='Halted'].id" --output text)
for id in $ids; do xo vm start "$id" --wait; done
```

Tag every VM whose name starts with `web-`:

```sh
for id in $(xo vm list --query "[?starts_with(name_label, 'web-')].id" --output text); do
  xo vm tag add "$id" web
done
```

## Destructive bulk actions

Commands such as `xo vm delete` ask for confirmation. In a loop, stdin is
usually not a terminal and they refuse with `confirmation required: re-run
with --yes to proceed non-interactively`. Pass `--yes` only after the dry
run, and keep the selection in a file you can review:

```sh
xo vm list --query "[?contains(tags, 'to-delete') && power_state=='Halted'].id" --output text > to-delete.txt
cat to-delete.txt                 # review
while read -r id; do xo vm delete "$id" --yes; done < to-delete.txt
```

## Undo

- Stopped VMs: start them again with the loop above.
- Tags: `xo vm tag remove <vm-id> <tag>` in the same kind of loop.
- Deleted VMs cannot be recovered.

## Troubleshooting

| Symptom | Likely cause / fix |
| ------- | ------------------ |
| `invalid --query expression "...": SyntaxError: ...` | Shell quoting broke the expression. Use double quotes outside, single quotes inside. |
| `invalid --query expression "...": invalid type for: <nil>, expected: ...` | A function got a field that does not exist (a typo, or a field of another resource). Check the field names with `--output json`. |
| `invalid --query expression "...": variable not defined: $poolId` | A `$` field name was not double-quoted in JMESPath. |
| The selection is empty | Compare with `--output json`: field names and values are case-sensitive (`Running`, not `running`). |
| Fewer results than expected | `--limit` was applied before the filter. Remove it. |
| `invalid VM id "web-01" (expected a UUID)` | Commands take UUIDs. Select `.id`, not `.name_label`. |
