package pool

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
		Short: "List pools",
		Long: `List XenServer pools from Xen Orchestra.

Examples:
  xo pool list
  xo pool list --output json
  xo pool list --query '[].name_label'
  xo pool list --limit 10`,
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

			pools, err := xo.Pool().GetAll(cmd.Context(), limit, "")
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot list pools: %v", err), cfg.Insecure)
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
			return renderPools(cmd.OutOrStdout(), cmd.Context(), format, pools, query, resolver)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. '[].name_label'")
	flags.IntVar(&limit, flagLimit, 0, "maximum number of pools to return (0 for no limit)")

	return cmd
}

// boolText renders a boolean as yes/no, the human convention shared by every
// list column and detail sheet that shows one.
func boolText(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// renderPools applies the optional --query expression and renders the result
// in the requested format. The human table shows each pool's master host by
// name (one batch lookup for every row, never one lookup per pool); the
// structured formats and --query still emit the raw objects, unchanged.
func renderPools(w io.Writer, ctx context.Context, format output.Format, pools []*payloads.Pool, query string, r *resolve.Client) error {
	queryResult, err := output.Query(query, pools)
	if err != nil {
		return err
	}

	var masters []uuid.UUID
	for _, p := range pools {
		if !p.Master.IsNil() {
			masters = append(masters, p.Master)
		}
	}
	masterNames := map[string]string{}
	if r != nil && len(masters) > 0 {
		masterNames = r.HostBatchNames(ctx, masters)
	}

	table := output.Table{
		Headers: []string{"ID", "NAME", "VERSION", "CORES", "SOCKETS", "MASTER", "HA"},
		Empty:   "pools",
	}
	for _, p := range pools {
		master := p.Master.String()
		if n, ok := masterNames[p.Master.String()]; ok {
			master = n
		}
		table.Rows = append(table.Rows, []string{
			p.ID.String(),
			p.NameLabel,
			p.PlatformVersion,
			fmt.Sprintf("%d", p.CPUs.Cores),
			fmt.Sprintf("%d", p.CPUs.Sockets),
			master,
			boolText(p.HAEnabled),
		})
	}

	// For structured formats without a query, normalize the SDK types so the
	// output only contains the requested data.
	var raw any = pools
	if format != output.FormatTable && (queryResult == nil || !queryResult.Present) {
		normalized, err := output.Normalize(pools)
		if err != nil {
			return err
		}
		raw = normalized
	}

	return output.Render(w, format, table, raw, queryResult)
}
