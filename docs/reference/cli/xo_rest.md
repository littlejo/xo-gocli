## xo rest

Call a raw Xen Orchestra REST endpoint

### Synopsis

Call a raw Xen Orchestra REST endpoint.

This is an escape hatch for endpoints that have no typed command yet (users
and groups, SR actions, ...), not a duplicate of the typed commands: if
'xo vm list' covers it, use 'xo vm list'. The request is sent
through the SDK v2 HTTP client (same authentication, base URL and TLS
handling), so this is not a second REST client. The OpenAPI spec at
<endpoint>/rest/v0/docs lists every endpoint and its fields; the REST API
documentation lives at https://docs.xen-orchestra.com/automation/restapi.

The <path> is relative to the REST API root (/rest/v0), so 'vdis' hits
/rest/v0/vdis.

Methods: get, post, put, patch, delete.
The request body (for post/put/patch) comes from --data as JSON; use --data -
to read it from stdin.

'xo rest delete' asks for confirmation; pass --yes to run non-interactively.

```
xo rest <method> <path> [flags]
```

### Examples

```
  xo rest get vdis --output json  # raw VDI fields (typed: 'xo vdi list')
  xo rest get vdis --param limit=10
  xo rest get vdis/<id>
  xo rest post srs/<id>/actions/scan       # action without a typed command
  xo rest get users --output json          # users have no typed command yet
  xo rest get vdis --output json --query '[].name_label'
  xo rest get vdis -i             # also print the status line and headers on stderr
```

### Options

```
      --data string      JSON request body for post/put/patch ('-' reads stdin)
      --header strings   extra request header KEY: VALUE (repeatable)
  -h, --help             help for rest
  -i, --include          print the status line and response headers on stderr
      --param strings    query parameter KEY=VALUE (repeatable)
  -q, --query string     JMESPath expression applied to the result, e.g. '[].name_label'
      --yes              do not ask for confirmation (or set XOA_YES=1)
```

### Options inherited from parent commands

```
  -d, --debug              show SDK/API error details on failure (or $XOA_DEBUG)
      --json               output as JSON (shorthand for --output json)
  -o, --output string      output format: table, json, yaml, text (or $XOA_DEFAULT_OUTPUT, or the profile's "output" value) (default "table")
  -p, --profile string     name of the configuration profile to use (or $XOA_PROFILE)
      --timeout duration   HTTP client timeout, e.g. 60s or 2m (default 30s, or $XOA_TIMEOUT)
```

### SEE ALSO

* [xo](xo.md)	 - Xen Orchestra command line client

