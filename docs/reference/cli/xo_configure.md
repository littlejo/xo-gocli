## xo configure

Manage configuration profiles

### Synopsis

Initialize or update a Xen Orchestra connection profile, or inspect and
manage the stored profiles.

Without a sub-command, 'xo configure' stores (or updates) the profile
selected with --profile: values can be provided with flags or interactively
(when stdin is a terminal). Environment variables take precedence over the
stored profile at run time.

Sub-commands:
  xo configure list           list the stored profiles
  xo configure show [name]    show one profile (the selected one by default)
  xo configure remove <name>  remove a stored profile

```
xo configure [flags]
```

### Examples

```
  xo configure
  xo configure --profile lab --endpoint https://xo.example.com --token <token>
  xo configure --profile lab --username admin --password <secret>
  xo configure list
  xo configure remove lab
```

### Options

```
      --endpoint string   Xen Orchestra endpoint, e.g. https://xo.example.com
  -h, --help              help for configure
  -k, --insecure          skip TLS certificate verification
      --password string   password (alternative to --token)
      --token string      authentication token (or use --username/--password)
      --username string   username (alternative to --token)
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
* [xo configure list](xo_configure_list.md)	 - List configuration profiles
* [xo configure remove](xo_configure_remove.md)	 - Remove a configuration profile
* [xo configure show](xo_configure_show.md)	 - Show a configuration profile

