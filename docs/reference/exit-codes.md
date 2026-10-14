# Exit codes

`xo` has two exit codes, so a script only needs to test for zero:

| Code | Meaning |
| ---- | ------- |
| `0`  | The command succeeded. With `--wait` or `xo task wait`, the task also ended in `success`. |
| `1`  | Any error: usage error (unknown command or flag, missing argument), configuration error (no profile, no endpoint), connection or authentication failure, API error, a confirmation refused or impossible without a terminal, a task that ended in `failure` or `interrupted` (with `--wait` or `xo task wait`), a wait deadline reached, or Ctrl+C. |

The error message is always on stderr, prefixed with `Error: `; stdout only
ever holds the requested data. Any other code (for example `2` with a Go
stack trace) is a crash: please [report it](https://github.com/littlejo/xo-gocli/issues)
with the output of `xo version` and the command run with `--debug`.

See [Timeouts and waiting](../getting-started/configuration.md#timeouts-and-waiting)
for how `--wait`, `xo task wait` and `--wait-timeout` decide when a wait ends.
