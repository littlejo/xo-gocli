# Security policy

## Supported versions

Security fixes are released for the latest minor version of the current
major version. Upgrade to the latest release before reporting an issue.

| Version         | Supported |
| --------------- | --------- |
| latest 1.x      | Yes       |
| older 1.x, 0.x  | No        |

## Reporting a vulnerability

Do not open a public issue. Report it privately through GitHub:
[Report a vulnerability](https://github.com/littlejo/xo-gocli/security/advisories/new).

Include the `xo version` output, your OS and architecture, the steps to
reproduce, and what an attacker could gain. Do not include real tokens,
passwords or endpoints; redact them in `--debug` output.

You will get an acknowledgement within 7 days. A fix is released as a patch
version, and the advisory is published once the fix is available.

## Scope

In scope:

- How `xo` handles credentials: tokens and passwords in the config file
  (written with `0600` permissions), in the environment, and in its output
  (`xo token list` and `xo configure list` mask secrets unless asked not to).
- TLS: certificate verification, and the `--insecure` / `$XOA_INSECURE`
  escape hatch, which must never be enabled implicitly.
- `install.sh` and the released archives and checksums.

Out of scope, to report upstream instead:

- Xen Orchestra itself: [vatesfr/xen-orchestra](https://github.com/vatesfr/xen-orchestra).
- The Go SDK: [vatesfr/xenorchestra-go-sdk](https://github.com/vatesfr/xenorchestra-go-sdk).
