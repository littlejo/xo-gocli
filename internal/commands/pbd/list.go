package pbd

import (
	"context"
	"fmt"
	"io"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

const (
	flagQuery = "query"
	flagLimit = "limit"
)

func newListCommand() *cobra.Command {
	var (
		query string
		limit int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List physical block devices (PBDs)",
		Long: `List physical block devices (PBDs) from Xen Orchestra.

A PBD is the connection between a host and a storage repository (SR); it is
what "plugs" an SR into a host. Use 'xo pbd plug' / 'xo pbd unplug' to
connect or disconnect it.

Examples:
  xo pbd list
  xo pbd list --output json
  xo pbd list --query '[].attached'
  xo pbd list --limit 10`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}
			if limit < 0 {
				return fmt.Errorf("--limit must be greater than or equal to 0")
			}
			if err := output.ValidateQuery(query); err != nil {
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

			pbds, err := xo.PBD().GetAll(cmd.Context(), limit, "")
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot list PBDs: %v", err), cfg.Insecure)
			}

			// Names (instead of raw UUIDs) are only shown in the human table;
			// --output json/yaml/text and --query keep the raw references, so
			// the resolver — and its extra batch requests — is only built for
			// the table.
			var resolver *resolve.Client
			if format == output.FormatTable && query == "" {
				resolver, err = cli.NewResolver(cmd, cfg)
				if err != nil {
					return err
				}
			}
			return renderPBDs(cmd.OutOrStdout(), cmd.Context(), format, pbds, query, resolver)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. '[].attached'")
	flags.IntVar(&limit, flagLimit, 0, "maximum number of PBDs to return (0 for no limit)")
	return cmd
}

// deviceOf returns the block device name from the PBD's device_config
// (e.g. "/dev/sda") or "-" when the PBD has no such entry (e.g. some network
// or iSCSI SRs use server/serverpath keys instead).
func deviceOf(pbd *payloads.PBD) string {
	if dev, ok := pbd.DeviceConfig["device"]; ok && dev != "" {
		return dev
	}
	return "-"
}

func boolText(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// renderPBDs applies the optional --query expression and renders the result in
// the requested format. The human table shows the host, SR and pool each PBD
// connects by name (at most three batch lookups for every row, never one
// lookup per PBD); the structured formats and --query still emit the raw
// objects, unchanged.
func renderPBDs(w io.Writer, ctx context.Context, format output.Format, pbds []*payloads.PBD, query string, r *resolve.Client) error {
	queryResult, err := output.Query(query, pbds)
	if err != nil {
		return err
	}

	var hosts, srs, pools []uuid.UUID
	for _, pbd := range pbds {
		if !pbd.Host.IsNil() {
			hosts = append(hosts, pbd.Host)
		}
		if !pbd.SR.IsNil() {
			srs = append(srs, pbd.SR)
		}
		if !pbd.Pool.IsNil() {
			pools = append(pools, pbd.Pool)
		}
	}
	hostNames := map[string]string{}
	if r != nil && len(hosts) > 0 {
		hostNames = r.HostBatchNames(ctx, hosts)
	}
	srNames := map[string]string{}
	if r != nil && len(srs) > 0 {
		srNames = r.SRBatchNames(ctx, srs)
	}
	poolNames := map[string]string{}
	if r != nil && len(pools) > 0 {
		poolNames = r.PoolBatchNames(ctx, pools)
	}

	table := output.Table{
		Headers: []string{"ID", "HOST", "SR", "POOL", "ATTACHED", "DEVICE"},
	}
	for _, pbd := range pbds {
		host := pbd.Host.String()
		if n, ok := hostNames[pbd.Host.String()]; ok {
			host = n
		}
		sr := pbd.SR.String()
		if n, ok := srNames[pbd.SR.String()]; ok {
			sr = n
		}
		pool := pbd.Pool.String()
		if n, ok := poolNames[pbd.Pool.String()]; ok {
			pool = n
		}
		table.Rows = append(table.Rows, []string{
			pbd.ID.String(),
			host,
			sr,
			pool,
			boolText(pbd.Attached),
			deviceOf(pbd),
		})
	}

	// For structured formats without a query, normalize the SDK types so the
	// output only contains the requested data.
	var raw any = pbds
	if format != output.FormatTable && (queryResult == nil || !queryResult.Present) {
		normalized, err := output.Normalize(pbds)
		if err != nil {
			return err
		}
		raw = normalized
	}

	return output.Render(w, format, table, raw, queryResult)
}
