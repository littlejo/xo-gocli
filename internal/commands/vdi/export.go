package vdi

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagVDIExportFile   = "file"
	flagVDIExportFormat = "format"
)

// vdiExportFormats are the disk image formats the XO REST API can export a VDI
// as.
var vdiExportFormats = map[string]payloads.VDIFormat{
	"raw": payloads.VDIFormatRaw,
	"vhd": payloads.VDIFormatVHD,
}

func newExportCommand() *cobra.Command {
	var (
		file   string
		format string
	)

	cmd := &cobra.Command{
		Use:   "export <id>",
		Short: "Export a virtual disk (VDI) to a raw or VHD image",
		Long: `Export a virtual disk (VDI) to a raw (default) or VHD image.

The image is streamed to stdout unless --file is given (use "-" for an
explicit stdout), so it can be piped:

  xo vdi export <id> > disk.raw
  xo vdi export <id> --file disk.raw

The VDI is referenced by its UUID, as returned by 'xo vdi list'.

The image is streamed over a single HTTP request, so the global HTTP client
timeout (--timeout, default 30s) bounds the whole transfer: a large disk over
a slow link can exceed it. Raise it with --timeout or $XOA_TIMEOUT when
exporting or importing large disks.

Examples:
  xo vdi export 11111111-1111-4111-8111-111111111111 --file disk.raw
  xo vdi export <id> > disk.raw
  xo vdi export <id> --format vhd --file disk.vhd`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vdiFormat, ok := vdiExportFormats[format]
			if !ok {
				return fmt.Errorf("invalid --format %q (expected raw or vhd)", format)
			}
			id, err := parseID(args[0])
			if err != nil {
				return err
			}

			xo, cfg, err := newClient(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// Existence check through the typed service, so a missing VDI
			// fails with the usual "not found" error before any download.
			vdi, err := xo.VDI().Get(ctx, id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}
			name := vdi.NameLabel
			if name == "" {
				name = id.String()
			}

			w, err := openExportOutput(cmd, file)
			if err != nil {
				return err
			}

			// The SDK service streams the image to the callback (and owns the
			// response close); the callback copies it to the destination.
			var n int64
			err = xo.VDI().Export(ctx, id, vdiFormat, func(r io.Reader) error {
				var err error
				n, err = io.Copy(w, r)
				if cerr := w.Close(); err == nil {
					err = cerr
				}
				return err
			})
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot export VDI %q: %v", name, err), cfg.Insecure)
			}

			return renderExported(cmd, name, exportDest(file), n)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&file, flagVDIExportFile, "", "destination file (default: stdout; use - to be explicit)")
	flags.StringVar(&format, flagVDIExportFormat, "raw", "image format: raw or vhd")
	return cmd
}

// openExportOutput opens the export destination. An empty or "-" file
// streams to the command's stdout; anything else is a regular file.
func openExportOutput(cmd *cobra.Command, file string) (io.WriteCloser, error) {
	if file == "" || file == "-" {
		return &stdoutCloser{w: cmd.OutOrStdout()}, nil
	}
	f, err := os.Create(file)
	if err != nil {
		return nil, fmt.Errorf("cannot create %q: %v", file, err)
	}
	return f, nil
}

// stdoutCloser adapts a plain io.Writer to io.WriteCloser for exports piped
// to stdout (there is nothing to close).
type stdoutCloser struct{ w io.Writer }

func (s *stdoutCloser) Write(p []byte) (int, error) { return s.w.Write(p) }
func (s *stdoutCloser) Close() error                { return nil }

// renderExported prints the outcome of a VDI export. When the image went to
// stdout, the message goes to stderr so stdout stays a clean binary stream.
func renderExported(cmd *cobra.Command, name, dest string, bytes int64) error {
	toStdout := dest == "stdout"
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	if toStdout {
		w = cmd.ErrOrStderr()
	}
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"vdi": name, "destination": dest, "bytes": bytes})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Exported VDI %q to %s (%d bytes)\n", name, dest, bytes)
		return err
	}
}

// exportDest reports where the image went: the destination file, or "stdout"
// when --file is empty or "-".
func exportDest(file string) string {
	if file == "" || file == "-" {
		return "stdout"
	}
	return file
}
