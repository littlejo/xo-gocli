package vdi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

const flagVDIImportFormat = "format"

var vdiImportFormats = map[string]payloads.VDIFormat{
	"raw": payloads.VDIFormatRaw,
	"vhd": payloads.VDIFormatVHD,
}

func newImportCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "import <id> <file>",
		Short: "Import a raw or VHD image into a virtual disk (VDI)",
		Long: `Import a raw (default) or VHD image into an existing virtual disk (VDI).

The image is read from <file>, or from stdin when <file> is "-". The VDI is
referenced by its UUID, as returned by 'xo vdi list', and its content is
OVERWRITTEN by the imported image:

  xo vdi import <id> disk.raw
  cat disk.raw | xo vdi import <id> -
  xo vdi import <id> disk.vhd --format vhd

This is a destructive operation and asks for confirmation unless --yes is
given.

The upload is a single HTTP request, so the global HTTP client timeout
(--timeout, default 30s) bounds the whole transfer: a large disk over a slow
link can exceed it. Raise it with --timeout or $XOA_TIMEOUT when importing
large disks.

Examples:
  xo vdi import 11111111-1111-4111-8111-111111111111 disk.raw
  xo vdi import <id> - --format vhd --yes < disk.vhd`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vdiFormat, ok := vdiImportFormats[format]
			if !ok {
				return fmt.Errorf("invalid --format %q (expected raw or vhd)", format)
			}
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			source := args[1]

			xo, cfg, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// Existence check through the typed service, so a missing VDI
			// fails with the usual "not found" error before any upload.
			vdi, err := xo.VDI().Get(ctx, id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}
			name := vdi.NameLabel
			if name == "" {
				name = id.String()
			}

			confirmed, err := confirm(cmd, fmt.Sprintf("Are you sure you want to overwrite VDI %q with %s?", name, source), cli.SkipConfirm(cmd))
			if err != nil {
				return err
			}
			if !confirmed {
				return errors.New("aborted")
			}

			body, size, err := openImportSource(cmd, source)
			if err != nil {
				return err
			}
			// The SDK Import requires an explicit content size; stdin has an
			// unknown length, so it is buffered before the request is sent.
			if size <= 0 {
				data, err := io.ReadAll(body)
				if cerr := body.Close(); err == nil {
					err = cerr
				}
				if err != nil {
					return cli.InsecureHint(fmt.Sprintf("cannot read %s: %v", source, err), cfg.Insecure)
				}
				body = io.NopCloser(bytes.NewReader(data))
				size = int64(len(data))
			}

			err = xo.VDI().Import(ctx, id, vdiFormat, body, size)
			// The HTTP layer closes the request body once it is sent, so this
			// defensive close can report "already closed" for a regular file;
			// that is harmless and must not be surfaced as a failure.
			_ = body.Close()
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot import into VDI %q: %v", name, err), cfg.Insecure)
			}

			return renderImported(cmd, name)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, flagVDIImportFormat, "raw", "image format: raw or vhd")
	// --yes (or $XOA_YES) skips the confirmation; the flag is read via
	// cli.SkipConfirm, which sees both the flag and the environment variable.
	flags.Bool(flagYes, false, "do not ask for confirmation (or set XOA_YES=1)")
	return cmd
}

// openImportSource opens the image source. A regular file is opened for
// reading (its size is reported as the Content-Length); "-" streams stdin
// with an unknown length (the caller buffers it).
func openImportSource(cmd *cobra.Command, file string) (io.ReadCloser, int64, error) {
	if file == "-" {
		return io.NopCloser(cmd.InOrStdin()), -1, nil
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot open %q: %v", file, err)
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("cannot stat %q: %v", file, err)
	}
	if fi.Size() <= 0 {
		_ = f.Close()
		return nil, 0, fmt.Errorf("%q is empty: the image cannot be empty", file)
	}
	return f, fi.Size(), nil
}

// renderImported prints the outcome of a VDI import.
func renderImported(cmd *cobra.Command, name string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"vdi": name})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Imported image into VDI %q\n", name)
		return err
	}
}
