# Work with several pools and profiles

One Xen Orchestra instance can manage many pools, and every `xo` command
already sees all the pools of the instance it is connected to. Profiles are
for the other case: several **Xen Orchestra instances** (production and lab,
one per site, one per customer), each with its own endpoint and credentials.

```text
   1. store one profile per instance     xo configure --profile <name>
   2. check what is stored               xo configure list / show
   3. pick a profile per command         --profile <name>
   4. pick a profile per shell           export XOA_PROFILE=<name>
   5. run a command on every profile     a shell loop
   6. clean up                           xo configure remove <name>
```

## Prerequisites

- An endpoint URL and a token (or a username and password) for each
  instance. See [Authentication](../getting-started/authentication.md) to
  create a token.

## Step 1: store one profile per instance

```sh
xo configure --profile prod --endpoint https://xo-prod.example.com --token <prod-token>
xo configure --profile lab --endpoint https://xo-lab.example.com --token <lab-token> --insecure
```

Each command prints `Profile "<name>" saved to <path>`. The profiles are
stored in `~/.config/xo/config`. `--insecure` (`-k`) skips TLS certificate
verification for that profile only, which is what you want for a lab with a
self-signed certificate.

Things to know:

- **The last profile you configure becomes the current one.** After the two
  commands above, `lab` is current. To make `prod` current again, re-run
  `xo configure --profile prod --endpoint <url> --token <token>` with its
  values.
- `xo configure` replaces the whole profile: pass every value again
  (including `--insecure`) when you update one.
- `xo configure` reads its target profile from `--profile` only, not from
  `$XOA_PROFILE`; without `--profile` it writes the `default` profile. Missing
  `--endpoint` and `--token` values are taken from `$XOA_ENDPOINT` and
  `$XOA_TOKEN` when they are set, then prompted for on a terminal.

## Step 2: check what is stored

```sh
xo configure list
```

The table has the columns `NAME CURRENT ENDPOINT AUTH INSECURE`; credentials
are masked. `xo configure show <name>` prints one profile in full, token or
password included, so do not paste its output anywhere. Without a name it
shows the selected profile.

## Step 3: pick a profile for one command

```sh
xo --profile prod vm list
xo vm list -p lab
```

`--profile` (`-p`) is a global flag: it can go before or after the command.

## Step 4: pick a profile for a shell session

```sh
export XOA_PROFILE=lab
xo vm list
```

## Which profile and which values win

The profile is selected in this order, first match wins:

1. `--profile` / `-p`;
2. `$XOA_PROFILE`;
3. the `current` profile of the configuration file;
4. the profile named `default`.

A selected profile that does not exist is an error
(`profile "lab" is not configured, run 'xo configure --profile lab'`).

Then, **environment variables override the values of the selected profile**:
`XOA_ENDPOINT`, `XOA_TOKEN`, `XOA_USERNAME`, `XOA_PASSWORD`, `XOA_INSECURE`
and `XOA_DEFAULT_OUTPUT`. A leftover `export XOA_ENDPOINT=...` in your shell
therefore sends `--profile prod` commands to another endpoint. When in doubt,
run with `--debug`: on failure it prints `debug: profile=<name>
endpoint=<url>`, the profile and endpoint that were actually used.

## Step 5: run a command on every profile

`xo configure list --query '[].name' --output text` prints one profile name
per line, which is all a loop needs:

```sh
for p in $(xo configure list --query '[].name' --output text); do
  echo "== $p"
  xo --profile "$p" vm list --power-state Running --query 'length(@)' || echo "   (failed)"
done
```

Each iteration is independent: an unreachable instance prints its `Error: ...`
on stderr and the loop continues with the next profile. To collect one JSON
document per instance instead:

```sh
for p in $(xo configure list --query '[].name' --output text); do
  xo --profile "$p" pool list --output json | jq --arg p "$p" '{profile: $p, pools: [.[].name_label]}'
done
```

## Step 6: remove a profile

```sh
xo configure remove lab --yes
```

Without `--yes` it asks for confirmation (or set `$XOA_YES`). When the removed
profile was the current one, the first remaining profile becomes current.

## Troubleshooting

| Symptom | Likely cause / fix |
| ------- | ------------------ |
| `profile "lab" is not configured, run 'xo configure --profile lab'` | `--profile` or `$XOA_PROFILE` names a profile that is not stored. Check `xo configure list` and `echo $XOA_PROFILE`. |
| Commands hit the wrong instance | An `XOA_ENDPOINT` / `XOA_TOKEN` variable overrides the profile. Run `env \| grep ^XOA_` and unset them. |
| The current profile changed by itself | Configuring a profile makes it current. Use `--profile` or `$XOA_PROFILE` explicitly in scripts. |
| `xo configure` updated `default` although `XOA_PROFILE=lab` is set | `xo configure` only honors `--profile`. Use `xo configure --profile lab --endpoint <url> --token <token>`. |
| TLS errors on one instance only | Its certificate is self-signed: store that profile again with `--insecure`. |
| `confirmation required: re-run with --yes to proceed non-interactively` | `xo configure remove` was run from a script. Add `--yes`. |

See also [Configuration](../getting-started/configuration.md) and
[Troubleshooting](../troubleshooting.md).
