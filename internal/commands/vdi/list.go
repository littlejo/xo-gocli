package vdi

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
	flagLimit = "limit"
	flagType  = "type"
)

func newListCommand() *cobra.Command {
	var (
		query   string
		limit   int
		vdiType string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List virtual disks (VDIs)",
		Long: `List virtual disks (VDIs) from Xen Orchestra.

A VDI is a virtual disk that lives on a storage repository (SR). Use
'xo vm vdis <id>' to list only the disks attached to a specific VM.

Examples:
  xo vdi list
  xo vdi list --output json
  xo vdi list --query '[].name_label'
  xo vdi list --type user
  xo vdi list --limit 10`,
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
			filter := vdiType
			if filter != "" {
				filter = "VDI_type:" + vdiType
			}

			cfg, err := config.Load(cli.ProfileName(cmd))
			if err != nil {
				return err
			}
			xo, err := cli.NewClient(cmd, cfg)
			if err != nil {
				return err
			}

			vdis, err := xo.VDI().GetAll(cmd.Context(), limit, filter)
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot list VDIs: %v", err), cfg.Insecure)
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
			return renderVDIs(cmd.OutOrStdout(), cmd.Context(), format, vdis, query, resolver)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. '[].name_label'")
	flags.IntVar(&limit, flagLimit, 0, "maximum number of VDIs to return (0 for no limit)")
	flags.StringVar(&vdiType, flagType, "", "filter by VDI type: user, system, suspend, …")

	return cmd
}

// renderVDIs applies the optional --query expression and renders the result
// in the requested format. The human table shows the SR each VDI lives on by
// name (one batch lookup for every row, never one lookup per VDI); the
// structured formats and --query still emit the raw objects, unchanged.
func renderVDIs(w io.Writer, ctx context.Context, format output.Format, vdis []*payloads.VDI, query string, r *resolve.Client) error {
	queryResult, err := output.Query(query, vdis)
	if err != nil {
		return err
	}

	var srs []uuid.UUID
	for _, vdi := range vdis {
		if !vdi.SR.IsNil() {
			srs = append(srs, vdi.SR)
		}
	}
	srNames := map[string]string{}
	if r != nil && len(srs) > 0 {
		srNames = r.SRBatchNames(ctx, srs)
	}

	table := output.Table{
		Headers: []string{"ID", "NAME", "TYPE", "SIZE", "USAGE", "SR"},
	}
	for _, vdi := range vdis {
		sr := vdi.SR.String()
		if n, ok := srNames[vdi.SR.String()]; ok {
			sr = n
		}
		table.Rows = append(table.Rows, []string{
			vdi.ID.String(),
			vdi.NameLabel,
			string(vdi.VDIType),
			units.HumanSize(float64(vdi.Size)),
			units.HumanSize(float64(vdi.Usage)),
			sr,
		})
	}

	// For structured formats without a query, normalize the SDK types so the
	// output only contains the requested data.
	var raw any = vdis
	if format != output.FormatTable && (queryResult == nil || !queryResult.Present) {
		normalized, err := output.Normalize(vdis)
		if err != nil {
			return err
		}
		raw = normalized
	}

	return output.Render(w, format, table, raw, queryResult)
}
