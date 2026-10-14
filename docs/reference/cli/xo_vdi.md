## xo vdi

Manage virtual disks (VDIs)

### Synopsis

Manage virtual disks (VDIs). A VDI is a disk that lives on a storage
repository (SR); it is not attached to a VM until a VBD links them (see
'xo vbd create').

VDIs are referenced by their UUID, as printed by 'xo vdi list'.

```
xo vdi [flags]
```

### Examples

```
  xo vdi list
  xo vdi create data --sr <sr-id> --size 20G
  xo vdi migrate <id> --sr <sr-id>
```

### Options

```
  -h, --help   help for vdi
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
* [xo vdi create](xo_vdi_create.md)	 - Create a virtual disk (VDI) on a storage repository
* [xo vdi delete](xo_vdi_delete.md)	 - Delete a virtual disk (VDI)
* [xo vdi export](xo_vdi_export.md)	 - Export a virtual disk (VDI) to a raw or VHD image
* [xo vdi get](xo_vdi_get.md)	 - Get a virtual disk (VDI)
* [xo vdi import](xo_vdi_import.md)	 - Import a raw or VHD image into a virtual disk (VDI)
* [xo vdi list](xo_vdi_list.md)	 - List virtual disks (VDIs)
* [xo vdi migrate](xo_vdi_migrate.md)	 - Migrate a virtual disk (VDI) to another storage repository
* [xo vdi tag](xo_vdi_tag.md)	 - Manage the tags of a virtual disk (VDI)

