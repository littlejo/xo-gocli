## xo version

Print the xo CLI version

### Synopsis

Print the version of the xo CLI.

This works offline, like 'aws version' or 'kubectl version --client'.
It prints the same line as '--version', so both can be used
interchangeably in scripts.

The Xen Orchestra server version is not shown: the REST API (v0) does not
expose it, and the CLI never uses the legacy JSON-RPC API.

```
xo version [flags]
```

### Options

```
  -h, --help   help for version
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

