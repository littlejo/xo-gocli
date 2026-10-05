package sr

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/go-units"
	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

func newGetCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a storage repository",
		Long: `Get a storage repository (SR) from Xen Orchestra, as a detailed,
human readable view.

The SR is referenced by its UUID, as returned by 'xo sr list'.

The human (default) view is a detail sheet that resolves the container by
name, shows the capacity (size, usage, physical usage) and the hosts the SR
is connected to (resolved in one batch, never one lookup per host). The
type, content type, allocation strategy, tags and the number of VDIs/PBDs
are shown too. If a reference cannot be resolved, the raw id is shown
instead and the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo sr get <id> --output json' is unchanged.

Examples:
  xo sr get 11111111-1111-4111-8111-111111111111
  xo sr get <id> --output json
  xo sr get <id> --query 'name_label'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if err := output.ValidateQuery(query); err != nil {
				return err
			}
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}

			xo, cfg, err := newClient(cmd)
			if err != nil {
				return err
			}

			sr, err := xo.SR().Get(cmd.Context(), id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}

			// The structured formats (json/yaml/text) and --query are served
			// straight from the raw object, exactly as before; only the default
			// human view is the new detail sheet, which resolves relationships.
			if format != output.FormatTable || query != "" {
				return renderSRRaw(cmd.OutOrStdout(), format, sr, query)
			}

			resolver, err := cli.NewResolver(cmd, cfg)
			if err != nil {
				return err
			}
			return renderSRDetail(cmd.OutOrStdout(), cmd.Context(), xo, sr, resolver)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'name_label'")
	return cmd
}

// parseID converts a positional SR identifier into a UUID.
func parseID(id string) (uuid.UUID, error) {
	u, err := uuid.FromString(id)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid SR id %q (expected a UUID)", id)
	}
	return u, nil
}

// newClient loads the selected profile and builds an authenticated SDK v2
// client. Commands must pass their cobra context to the SDK operations so that
// cancellation (Ctrl+C) reaches the HTTP layer.
func newClient(cmd *cobra.Command) (library.Library, *config.ClientConfig, error) {
	cfg, err := config.Load(cli.ProfileName(cmd))
	if err != nil {
		return nil, nil, err
	}
	xo, err := cli.NewClient(cmd, cfg)
	if err != nil {
		return nil, nil, err
	}
	return xo, cfg, nil
}

// notFound delegates to cli.NotFound, which reports a 404 concisely and keeps
// the raw API error as debug detail.
func notFound(id string, err error, insecure bool) error {
	return cli.NotFound("SR", "get", id, err, insecure)
}

// renderSRRaw renders the SR in the structured formats (json/yaml/text) or as
// a --query projection. It emits the raw object so machine readable output and
// the query pipeline are unchanged.
func renderSRRaw(w io.Writer, format output.Format, sr *payloads.StorageRepository, query string) error {
	if query != "" {
		queryResult, err := output.Query(query, sr)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, sr, queryResult)
	}

	normalized, err := output.Normalize(sr)
	if err != nil {
		return err
	}
	return output.Render(w, format, output.Table{}, normalized, nil)
}

// renderSRDetail renders the single SR as a human readable detail sheet. The
// container is resolved by name and the connected hosts are resolved in one
// batch (never one lookup per host), a constant cost. A reference that cannot
// be resolved falls back to its raw id, so the view is always complete.
func renderSRDetail(w io.Writer, ctx context.Context, xo library.Library, sr *payloads.StorageRepository, r *resolve.Client) error {
	lines := make([]string, 0, 14)

	// Header: type and name.
	header := "SR " + sr.NameLabel
	if sr.InMaintenanceMode {
		header += "  (maintenance)"
	}
	lines = append(lines, header)

	// Identity
	if sr.NameDescription != "" {
		lines = append(lines, output.DetailField("Description", sr.NameDescription))
	}
	if len(sr.Tags) > 0 {
		lines = append(lines, output.DetailField("Tags", strings.Join(sr.Tags, ", ")))
	}

	// Location (resolved by name; the resolver falls back to the raw id when a
	// reference cannot be resolved, so the view is always complete).
	if name, err := r.ResolveContainer(ctx, sr.Container); err == nil {
		lines = append(lines, output.DetailField("Container", name))
	} else {
		lines = append(lines, output.DetailField("Container", sr.Container.String()))
	}

	// Storage
	lines = append(lines, output.DetailField("Type", output.OrDash(sr.SRType)))
	lines = append(lines, output.DetailField("Content", output.OrDash(sr.ContentType)))
	lines = append(lines, output.DetailField("Shared", boolText(sr.Shared)))
	if sr.AllocationStrategy != "" && sr.AllocationStrategy != payloads.AllocationStrategyUnknown {
		lines = append(lines, output.DetailField("Allocation", string(sr.AllocationStrategy)))
	}
	lines = append(lines, output.DetailField("Size", sizeText(sr.Size)))
	lines = append(lines, output.DetailField("Usage", usageText(sr)))
	if sr.PhysicalUsage > 0 {
		lines = append(lines, output.DetailField("Physical", sizeText(sr.PhysicalUsage)))
	}

	// Contents
	lines = append(lines, output.DetailField("VDIs", fmt.Sprintf("%d", len(sr.VDIs))))
	if len(sr.PBDs) > 0 {
		lines = append(lines, output.DetailField("PBDs", fmt.Sprintf("%d", len(sr.PBDs))))
	}

	// Relationships (the hosts the SR is connected to, resolved in one batch).
	if connected := connectedHosts(ctx, xo, sr, r); connected != "" {
		lines = append(lines, output.DetailField("Connected to", connected))
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// connectedHosts returns the names of the hosts the SR is connected to,
// resolved in one batch. It is best-effort: when the PBDs cannot be fetched
// the line is simply omitted, so the detail view never fails because of it.
// The cost is constant (one PBD list filtered by the SR, one host name
// batch), never one lookup per host.
func connectedHosts(ctx context.Context, xo library.Library, sr *payloads.StorageRepository, r *resolve.Client) string {
	if len(sr.PBDs) == 0 {
		return ""
	}
	pbds, err := xo.PBD().GetAll(ctx, 0, "SR:"+sr.ID.String())
	if err != nil || len(pbds) == 0 {
		return ""
	}
	hostIDs := make([]uuid.UUID, 0, len(pbds))
	for _, p := range pbds {
		if !p.Host.IsNil() {
			hostIDs = append(hostIDs, p.Host)
		}
	}
	if len(hostIDs) == 0 {
		return ""
	}
	names := r.HostBatchNames(ctx, hostIDs)
	out := make([]string, 0, len(hostIDs))
	for _, id := range hostIDs {
		out = append(out, names[id.String()])
	}
	return output.JoinNames(out)
}

func sizeText(bytes float64) string {
	if bytes == 0 {
		return "-"
	}
	return units.HumanSize(bytes)
}

// usageText renders "used / total (percent)" so the free space is visible at
// a glance, or just the used amount when the total is unknown.
func usageText(sr *payloads.StorageRepository) string {
	if sr.Size == 0 {
		return sizeText(sr.Usage)
	}
	pct := int(100 * sr.Usage / sr.Size)
	return fmt.Sprintf("%s / %s (%d%%)", sizeText(sr.Usage), sizeText(sr.Size), pct)
}

func boolText(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
