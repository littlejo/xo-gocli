package host

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/go-units"
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
		Short: "List hosts",
		Long: `List XenServer hosts from Xen Orchestra.

Examples:
  xo host list
  xo host list --output json
  xo host list --query '[].name_label'
  xo host list --limit 10`,
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

			hosts, err := xo.Host().GetAll(cmd.Context(), limit, "")
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot list hosts: %v", err), cfg.Insecure)
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
			return renderHosts(cmd.OutOrStdout(), cmd.Context(), format, hosts, query, resolver)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. '[].name_label'")
	flags.IntVar(&limit, flagLimit, 0, "maximum number of hosts to return (0 for no limit)")

	return cmd
}

// renderHosts applies the optional --query expression and renders the result
// in the requested format. The human table shows the pool each host belongs
// to by name (one batch lookup for every row, never one lookup per host); the
// structured formats and --query still emit the raw objects, unchanged.
func renderHosts(w io.Writer, ctx context.Context, format output.Format, hosts []*payloads.Host, query string, r *resolve.Client) error {
	queryResult, err := output.Query(query, hosts)
	if err != nil {
		return err
	}

	var pools []uuid.UUID
	for _, h := range hosts {
		if !h.Pool.IsNil() {
			pools = append(pools, h.Pool)
		}
	}
	poolNames := map[string]string{}
	if r != nil && len(pools) > 0 {
		poolNames = r.PoolBatchNames(ctx, pools)
	}

	table := output.Table{
		Headers: []string{"ID", "NAME", "ADDRESS", "POWER STATE", "PLATFORM", "MEMORY", "VMS", "POOL"},
	}
	for _, h := range hosts {
		pool := h.Pool.String()
		if n, ok := poolNames[h.Pool.String()]; ok {
			pool = n
		}
		table.Rows = append(table.Rows, []string{
			h.ID.String(),
			h.NameLabel,
			h.Address,
			h.PowerState,
			h.Version,
			memoryText(h),
			fmt.Sprintf("%d", len(h.ResidentVMs)),
			pool,
		})
	}

	// For structured formats without a query, normalize the SDK types so the
	// output only contains the requested data.
	var raw any = hosts
	if format != output.FormatTable && (queryResult == nil || !queryResult.Present) {
		normalized, err := output.Normalize(hosts)
		if err != nil {
			return err
		}
		raw = normalized
	}

	return output.Render(w, format, table, raw, queryResult)
}

func memoryText(h *payloads.Host) string {
	if h.Memory == nil || h.Memory.Size == 0 {
		return ""
	}
	return units.HumanSize(float64(h.Memory.Size))
}
