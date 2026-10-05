package vm

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagExportFile   = "file"
	flagExportFormat = "format"
	flagCompress     = "compress"
)

// xvaExportFormats are the archive formats the XO REST API can export a VM as.
var xvaExportFormats = map[string]bool{"xva": true, "ova": true}

func newExportCommand() *cobra.Command {
	var (
		file     string
		format   string
		compress bool
	)

	cmd := &cobra.Command{
		Use:   "export <id>",
		Short: "Export a virtual machine to an XVA or OVA archive",
		Long: `Export a virtual machine to an XVA (default) or OVA archive.

The archive is streamed to stdout unless --file is given (use "-" for an
explicit stdout), so it can be piped:

  xo vm export <id> > vm.xva
  xo vm export <id> --file vm.xva

The VM is referenced by its UUID, as returned by 'xo vm list'.

This operation is not exposed by the SDK v2 typed service yet, so it is
performed through the SDK's own REST client against GET /vms/<id>.<format>
(documented in xva.go; to contribute upstream).

The archive is streamed over a single HTTP request, so the global HTTP
client timeout (--timeout, default 30s) bounds the whole transfer: a large
XVA or OVA over a slow link can exceed it. Raise it with --timeout or
$XOA_TIMEOUT when exporting or importing large archives.

Examples:
  xo vm export 550e8400-e29b-41d4-a716-446655440001 --file web-01.xva
  xo vm export <id> > web-01.xva
  xo vm export <id> --format ova --file web-01.ova
  xo vm export <id> --compress=false`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !xvaExportFormats[format] {
				return fmt.Errorf("invalid --format %q (expected xva or ova)", format)
			}
			id, err := parseID(args[0])
			if err != nil {
				return err
			}

			cfg, err := config.Load(cli.ProfileName(cmd))
			if err != nil {
				return err
			}
			xo, err := cli.NewClient(cmd, cfg)
			if err != nil {
				return err
			}
			// Existence check through the typed service, so a missing VM
			// fails with the usual "not found" error before any download.
			vm, err := xo.VM().GetByID(cmd.Context(), id)
			if err != nil {
				return cli.NotFound("VM", "resolve", args[0], err, cfg.Insecure)
			}

			httpClient, err := cli.NewHTTPClient(cmd, cfg)
			if err != nil {
				return err
			}

			w, err := openExportOutput(cmd, file)
			if err != nil {
				return err
			}

			query := url.Values{}
			if format == "xva" {
				query.Set("compress", fmt.Sprintf("%t", compress))
			}
			resp, err := xvRequest(cmd.Context(), httpClient, "GET", "vms/"+id.String()+"."+format, query, nil, "", -1)
			if err != nil {
				_ = w.Close()
				return cli.InsecureHint(fmt.Sprintf("cannot export VM %q: %v", vm.NameLabel, err), cfg.Insecure)
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				apiErr := xvaAPIError(resp)
				_ = resp.Body.Close()
				_ = w.Close()
				return cli.InsecureHint(fmt.Sprintf("cannot export VM %q: %v", vm.NameLabel, apiErr), cfg.Insecure)
			}

			n, err := io.Copy(w, resp.Body)
			closeErr := resp.Body.Close()
			if werr := w.Close(); closeErr == nil {
				closeErr = werr
			}
			if err == nil {
				err = closeErr
			}
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot export VM %q: %v", vm.NameLabel, err), cfg.Insecure)
			}

			return renderExported(cmd, vm.NameLabel, exportDest(file), n)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&file, flagExportFile, "", "destination file (default: stdout; use - to be explicit)")
	flags.StringVar(&format, flagExportFormat, "xva", "archive format: xva or ova")
	flags.BoolVar(&compress, flagCompress, true, "compress the archive (xva only)")
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

// renderExported prints the outcome of a VM export. When the archive went to
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
		raw, err := output.Normalize(map[string]any{"vm": name, "destination": dest, "bytes": bytes})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Exported VM %q to %s (%d bytes)\n", name, dest, bytes)
		return err
	}
}

// exportDest reports where the archive went: the destination file, or
// "stdout" when --file is empty or "-".
func exportDest(file string) string {
	if file == "" || file == "-" {
		return "stdout"
	}
	return file
}
