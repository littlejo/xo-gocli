## xo configure

Manage configuration profiles

### Synopsis

Initialize or update a Xen Orchestra connection profile, or inspect and
manage the stored profiles.

Without a sub-command, 'xo configure' stores the selected profile: the one
named by --profile, else $XOA_PROFILE, else the current profile, else
"default". Values come from the flags, then from the XOA_* environment
variables, then from an interactive prompt (when stdin is a terminal).

Updating an existing profile only changes what you give: the other stored
values (endpoint, credentials, --insecure, the "output" setting) are kept.
Giving a token replaces a stored username and password, and the reverse.
Use --insecure=false to turn TLS verification back on.

The first profile you store becomes the current one. Storing another profile
does not change the current one: select it per command with --profile or
$XOA_PROFILE, or make it current with 'xo configure use <name>'.

```
xo configure [flags]
```

### Examples

```
  xo configure
  xo configure --profile lab --endpoint https://xo.example.com --token <token>
  xo configure --profile lab --username admin --password <secret>
  xo configure --profile lab --token <new-token>
  xo configure use lab
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
* [xo configure use](xo_configure_use.md)	 - Make a profile the current one

