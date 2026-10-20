package host

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"

	"github.com/gofrs/uuid"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

func newGetCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "get <id|name>",
		Short: "Get a host",
		Long: `Get a host from Xen Orchestra, as a detailed, human readable view.

The host is referenced by its UUID, as returned by 'xo host list'.

The host can also be given by its exact name (name_label), at the cost of
one more request; a name shared by several hosts is an error that lists
their ids.

The human (default) view is a detail sheet: the address, the pool the host
belongs to (shown by name), the memory usage, the CPU model and the VMs
resident on the host (resolved in one batch, never one lookup per VM).
Platform, license and status fields are shown too. If the pool cannot be
resolved, the raw id is shown instead and the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo host get <id> --output json' is unchanged.`,
		Example: `  xo host get 550e8400-e29b-41d4-a716-446655440001
  xo host get xcp-host-01
  xo host get <id> --output json
  xo host get <id> --query 'name_label'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := output.ValidateQuery(query); err != nil {
				return err
			}
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}

			xo, cfg, err := cli.LoadClient(cmd)
			if err != nil {
				return err
			}
			id, err := lookupID(cmd.Context(), xo, cfg.Insecure, args[0])
			if err != nil {
				return err
			}

			host, err := xo.Host().Get(cmd.Context(), id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}

			// The structured formats (json/yaml/text) and --query are served
			// straight from the raw object, exactly as before; only the default
			// human view is the new detail sheet, which resolves relationships.
			if format != output.FormatTable || query != "" {
				return output.RenderRaw(cmd.OutOrStdout(), format, host, query)
			}

			resolver, err := cli.NewResolver(cmd, cfg)
			if err != nil {
				return err
			}
			return renderHostDetail(cmd.OutOrStdout(), cmd.Context(), host, resolver)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'name_label'")
	return cmd
}

// notFound delegates to cli.NotFound, which reports a 404 concisely and keeps
// the raw API error as debug detail.
func notFound(id string, err error, insecure bool) error {
	return cli.NotFound("host", "get", id, err, insecure)
}

// renderHostDetail renders the single host as a human readable detail sheet.
// The pool is resolved by name (a single constant lookup) and the resident VMs
// are resolved in one batch (never one lookup per VM). A reference that cannot
// be resolved falls back to its raw id, so the view is always complete.
func renderHostDetail(w io.Writer, ctx context.Context, h *payloads.Host, r *resolve.Client) error {
	lines := make([]string, 0, 18)

	// Header: name and (power state).
	header := "Host " + h.NameLabel
	if h.PowerState != "" {
		header += "  (" + h.PowerState + ")"
	}
	lines = append(lines, header)

	// Identity
	if h.NameDescription != "" {
		lines = append(lines, output.DetailField("Description", h.NameDescription))
	}
	if len(h.Tags) > 0 {
		lines = append(lines, output.DetailField("Tags", strings.Join(h.Tags, ", ")))
	}

	// Location (resolved by name; the resolver falls back to the raw id when
	// the pool cannot be resolved, so the view is always complete).
	if !h.Pool.IsNil() {
		name, _ := r.PoolName(ctx, h.Pool)
		lines = append(lines, output.DetailField("Pool", output.OrDash(name)))
	}
	lines = append(lines, output.DetailField("Address", output.OrDash(h.Address)))
	if h.Hostname != "" {
		lines = append(lines, output.DetailField("Hostname", h.Hostname))
	}

	// Resources
	if m := h.Memory; m != nil && m.Size > 0 {
		pct := int(100 * m.Usage / m.Size)
		lines = append(lines, output.DetailField("Memory", fmt.Sprintf("%s / %s (%d%%)",
			units.HumanSize(float64(m.Usage)), units.HumanSize(float64(m.Size)), pct)))
	}
	if c := h.HostCPUCores; c != nil && (c.Cores > 0 || c.Sockets > 0) {
		lines = append(lines, output.DetailField("CPUs", fmt.Sprintf("%d cores, %d sockets", c.Cores, c.Sockets)))
	}
	if ci := h.CPUs; ci != nil && ci.ModelName != "" {
		model := ci.ModelName
		if ci.Speed != "" {
			model += " @ " + ci.Speed + " MHz"
		}
		lines = append(lines, output.DetailField("CPU", model))
	}

	// Platform
	if h.Version != "" {
		platform := h.ProductBrand + " " + h.Version
		if h.Build != "" {
			platform += " (" + h.Build + ")"
		}
		lines = append(lines, output.DetailField("Platform", strings.TrimSpace(platform)))
	}
	if h.LicenseExpiry != nil {
		lines = append(lines, output.DetailField("License", "expires "+licenseExpiryText(*h.LicenseExpiry)))
	}

	// Workload (the resident VMs, resolved in one batch).
	if len(h.ResidentVMs) > 0 {
		lines = append(lines, output.DetailField("VMs", residentVMs(ctx, h.ResidentVMs, r)))
	}

	// Status
	if h.RebootRequired {
		lines = append(lines, output.DetailField("Reboot", "required"))
	}
	if !h.Enabled {
		lines = append(lines, output.DetailField("Enabled", "no"))
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// residentVMs renders the count plus the (batch-resolved) names of the VMs
// resident on the host, so the sheet shows "n" when few and stays bounded
// when many. The names come from a single VM batch, never one lookup per VM.
func residentVMs(ctx context.Context, ids []uuid.UUID, r *resolve.Client) string {
	count := fmt.Sprintf("%d", len(ids))
	names := r.VMBatchNames(ctx, ids)
	ordered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id.IsNil() {
			continue
		}
		ordered = append(ordered, names[id.String()])
	}
	if len(ordered) == 0 {
		return count
	}
	return count + "  (" + output.JoinNames(ordered) + ")"
}

// licenseExpiryText renders a Unix-seconds license expiry as a short date, or
// the raw number when it does not look like a timestamp.
func licenseExpiryText(ts int64) string {
	if ts <= 0 {
		return fmt.Sprintf("%d", ts)
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02")
}

// lookupID resolves the host a user referenced by UUID or by exact name
// (see cli.LookupID).
func lookupID(ctx context.Context, xo library.Library, insecure bool, ref string) (uuid.UUID, error) {
	return cli.LookupID(ctx, "host", ref, insecure,
		func(ctx context.Context, filter string) ([]*payloads.Host, error) {
			return xo.Host().GetAll(ctx, 0, filter)
		},
		func(o *payloads.Host) uuid.UUID { return o.ID },
		func(o *payloads.Host) string { return o.NameLabel })
}
