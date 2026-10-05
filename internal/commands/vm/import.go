package vm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagImportSR = "sr"
)

func newImportCommand() *cobra.Command {
	var (
		poolID string
		srID   string
	)

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import a virtual machine from an XVA archive",
		Long: `Import a virtual machine from an XVA archive into a pool.

The XVA file is read from <file>, or from stdin when <file> is "-". The
--pool flag is required and selects the pool the VM is imported into
(see 'xo pool list'); --sr optionally pins the VM's disks to a specific
storage repository of that pool.

This operation is not exposed by the SDK v2 typed service yet, so it is
performed through the SDK's own REST client against POST /pools/<pool>/vms
(documented in xva.go; to contribute upstream).

The upload is a single HTTP request, so the global HTTP client timeout
(--timeout, default 30s) bounds the whole transfer: a large XVA over a slow
link can exceed it. Raise it with --timeout or $XOA_TIMEOUT when importing
large archives.

Examples:
  xo vm import web-01.xva --pool 6b7c8d9e-0000-1111-2222-333344445555
  xo vm import web-01.xva --pool <pool-id> --sr <sr-id>
  cat web-01.xva | xo vm import - --pool <pool-id>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if poolID == "" {
				return fmt.Errorf("--pool is required (see 'xo pool list')")
			}
			pool, err := uuid.FromString(poolID)
			if err != nil {
				return fmt.Errorf("invalid --pool id %q (expected a UUID)", poolID)
			}
			if srID != "" {
				if _, err := uuid.FromString(srID); err != nil {
					return fmt.Errorf("invalid --sr id %q (expected a UUID)", srID)
				}
			}

			cfg, err := config.Load(cli.ProfileName(cmd))
			if err != nil {
				return err
			}
			httpClient, err := cli.NewHTTPClient(cmd, cfg)
			if err != nil {
				return err
			}

			// Existence check for the target pool through the typed service.
			xo, err := cli.NewClient(cmd, cfg)
			if err != nil {
				return err
			}
			if _, err := xo.Pool().Get(cmd.Context(), pool); err != nil {
				return cli.NotFound("pool", "resolve", poolID, err, cfg.Insecure)
			}

			body, contentLength, err := openImportSource(cmd, args[0])
			if err != nil {
				return err
			}

			query := url.Values{}
			if srID != "" {
				query.Set("sr", srID)
			}
			resp, err := xvRequest(cmd.Context(), httpClient, "POST", "pools/"+pool.String()+"/vms", query, body, "application/octet-stream", contentLength)
			// net/http takes ownership of the request body and closes it once
			// the request is sent, so this is a defensive close that can report
			// "already closed" for a regular file; that is harmless and must not
			// be surfaced as a failure.
			_ = body.Close()
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot import VM from %q: %v", args[0], err), cfg.Insecure)
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				apiErr := xvaAPIError(resp)
				_ = resp.Body.Close()
				return cli.InsecureHint(fmt.Sprintf("cannot import VM from %q: %v", args[0], apiErr), cfg.Insecure)
			}

			var result struct {
				ID string `json:"id"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				_ = resp.Body.Close()
				return fmt.Errorf("cannot read import response: %v", err)
			}
			if cerr := resp.Body.Close(); cerr != nil {
				return fmt.Errorf("cannot read import response: %v", cerr)
			}
			if result.ID == "" {
				return fmt.Errorf("import response did not include the new VM id")
			}

			return renderImported(cmd, result.ID)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&poolID, flagPool, "", "pool UUID to import the VM into (see 'xo pool list')")
	flags.StringVar(&srID, flagImportSR, "", "SR UUID of the pool to pin the VM's disks to (default: pool default)")
	return cmd
}

// openImportSource opens the XVA source. A regular file is opened for
// reading (its size is reported as the Content-Length); "-" streams stdin
// with an unknown length.
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
		return nil, 0, fmt.Errorf("%q is empty: the XVA archive cannot be empty", file)
	}
	return f, fi.Size(), nil
}

// renderImported prints the outcome of a VM import: a friendly line for the
// human formats and the new VM id for the machine formats.
func renderImported(cmd *cobra.Command, id string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"vm": id})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Imported VM %s\n", id)
		return err
	}
}
