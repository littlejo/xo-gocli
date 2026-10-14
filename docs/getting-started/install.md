# Install `xo`

`xo` is a single static binary for Linux, macOS and Windows, on amd64 and
arm64 (the SDK does not compile on 32-bit). Once it is installed,
[configure a profile](configuration.md).

- [Install script (Linux and macOS)](#install-script-linux-and-macos)
- [Manual download](#manual-download)
- [Windows](#windows)
- [Man pages](#man-pages)
- [From source](#from-source)
- [Upgrade](#upgrade)
- [Uninstall](#uninstall)

## Install script (Linux and macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/littlejo/xo-gocli/main/install.sh | sh
```

The script detects your OS and architecture (Linux or macOS, amd64 or arm64),
downloads the latest release artifact and installs the binary to
`/usr/local/bin` (or `~/.local/bin` if you don't have write access). It
verifies the archive's SHA-256 checksum when `sha256sum` or `shasum` is
available; without either, it prints a warning and installs **without**
verifying, so check the archive by hand in that case (see
[Manual download](#manual-download)).

Three environment variables change its defaults:

| Variable              | Default                                   |
| --------------------- | ----------------------------------------- |
| `XOA_INSTALL_VERSION` | the latest release, e.g. `v1.0.0` to pin  |
| `XOA_INSTALL_DIR`     | `/usr/local/bin`, then `~/.local/bin`     |
| `XOA_INSTALL_REPO`    | `littlejo/xo-gocli`                       |

```sh
curl -fsSL https://raw.githubusercontent.com/littlejo/xo-gocli/main/install.sh \
  | XOA_INSTALL_VERSION=v1.0.0 XOA_INSTALL_DIR="$HOME/bin" sh
```

The script does not support Windows: use the [manual download](#windows).

## Manual download

Download `xo_<version>_<os>_<arch>.tar.gz` (or `.zip` on Windows) from the
[Releases page](https://github.com/littlejo/xo-gocli/releases), verify it against
`xo_<version>_checksums.txt`, extract it and put the `xo` binary on your
`PATH`.

## Windows

There is no install script for Windows. Download
`xo_<version>_windows_<arch>.zip` from the
[Releases page](https://github.com/littlejo/xo-gocli/releases), verify it
against `xo_<version>_checksums.txt`, extract it, and put `xo.exe` in a
directory listed in your `PATH`.

## Man pages

Each archive also holds the man pages, one per command, under `man/`. To
install them on Linux or macOS:

```sh
sudo install -m 0644 man/*.1 /usr/local/share/man/man1/
man xo-vm-start
```

The same content is available online as the
[command reference](../reference/cli/xo.md).

## From source

Requires Go 1.26+ (a [mise](https://mise.jdx.dev/) config is included):

```sh
git clone https://github.com/littlejo/xo-gocli.git
cd xo-gocli
go build -o dist/xo ./cmd/xo
```

See [CONTRIBUTING.md](../../CONTRIBUTING.md#toolchain) for the full toolchain.

## Upgrade

Re-run the install script: it installs the latest release over the existing
binary. To move to a specific version instead, pin it:

```sh
curl -fsSL https://raw.githubusercontent.com/littlejo/xo-gocli/main/install.sh \
  | XOA_INSTALL_VERSION=v1.0.0 sh
```

If you installed to a custom directory, pass the same `XOA_INSTALL_DIR`
again. For a manual install, download the new archive and replace the binary
(and the man pages). Check the result with `xo version`, and read the
[changelog](../../CHANGELOG.md) for what changed.

## Uninstall

Remove the binary, the man pages and, if you no longer need them, the stored
profiles:

```sh
rm "$(command -v xo)"
sudo rm /usr/local/share/man/man1/xo.1 /usr/local/share/man/man1/xo-*.1
rm -r ~/.config/xo
```

`~/.config/xo` holds your profiles, including any stored token or password.
If you set `$XOA_CONFIG_FILE`, remove that file instead. Remove the shell
completion script too if you [installed one](shell-completion.md).
