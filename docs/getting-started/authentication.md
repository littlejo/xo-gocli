# Authentication

`xo` authenticates to Xen Orchestra with a **token** or with a **username and
password**. One of the two must be available, from the
[profile](configuration.md) or from the environment (`XOA_TOKEN`, or
`XOA_USERNAME` + `XOA_PASSWORD`).

- [Getting a token](#getting-a-token)
- [Token or password?](#token-or-password)

## Getting a token

A token is the recommended way to authenticate (see
[Token or password?](#token-or-password) for why). Xen Orchestra issues it to
a user, and it carries that user's permissions. Pick one of these ways to get
the first one:

- **With `xo` itself**, starting from a username and password. The password is
  prompted, so it stays out of your shell history. Store the token printed by
  `xo token create` in the `lab` profile, then drop the temporary one:

  ```sh
  xo configure --profile bootstrap --endpoint https://xo.example.com --username admin
  xo token create --profile bootstrap --description "laptop" --expires-in "90 days"
  xo configure --profile lab --endpoint https://xo.example.com --token <token>
  xo configure remove bootstrap --yes
  ```

- **From the Xen Orchestra web UI**, in your user settings (create a new
  authentication token there and copy it).
- **With the Node.js `xo-cli`**: `xo-cli create-token xo.example.com admin`
  prompts for the password and prints a token
  ([XO REST API docs](https://docs.xen-orchestra.com/automation/restapi)).

The token is printed only once: store it right away with `xo configure`.
Tokens expire (the server's `maxTokenValidity` sets the upper bound); when one
does, commands fail with an authentication error and you create a new one the
same way.

To list or inspect the tokens of the current user afterwards, see
[`xo token`](../reference/commands.md#xo-token).

## Token or password?

**Token profiles are recommended.** A token profile authenticates with the
cookie XO already issued, so no login request is sent at all. With a
username + password profile, the SDK logs in once per client it builds
(`POST /auth/login`), and a command that needs both the typed SDK services
and the raw REST client (the `list`/`get` tables resolve names, so most of
them do) builds two of them — two logins per invocation. This is a known
inefficiency tracked for v1.1 (a single shared client); until then, token
profiles avoid the duplicate logins and the resulting auth load on the
server.

A token is also easier to rotate and to scope: create one per machine or per
CI pipeline, with an expiry, and never put it on the command line in shared
scripts — pass it through `XOA_TOKEN` instead (see
[Environment variables](configuration.md#environment-variables)).
