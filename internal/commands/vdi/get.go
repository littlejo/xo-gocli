package vdi

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

const flagQuery = "query"

func newGetCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a virtual disk (VDI)",
		Long: `Get a virtual disk (VDI) from Xen Orchestra, as a detailed,
human readable view.

The VDI is referenced by its UUID, as returned by 'xo vdi list' or
'xo vm vdis <id>'.

The human (default) view is a detail sheet that resolves the SR by name and
lists the VMs the disk is attached to (resolved in one batch, never one
lookup per VM). The disk's type, size, usage, snapshots and tags are shown
too. If a reference cannot be resolved, the raw id is shown instead and the
command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo vdi get <id> --output json' is unchanged.

Examples:
  xo vdi get 11111111-1111-4111-8111-111111111111
  xo vdi get <id> --output json
  xo vdi get <id> --query 'name_label'`,
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

			vdi, err := xo.VDI().Get(cmd.Context(), id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}

			// The structured formats (json/yaml/text) and --query are served
			// straight from the raw object, exactly as before; only the default
			// human view is the new detail sheet, which resolves relationships.
			if format != output.FormatTable || query != "" {
				return renderVDIRaw(cmd.OutOrStdout(), format, vdi, query)
			}

			resolver, err := cli.NewResolver(cmd, cfg)
			if err != nil {
				return err
			}
			return renderVDIDetail(cmd.OutOrStdout(), cmd.Context(), xo, vdi, resolver)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'name_label'")
	return cmd
}

// parseID converts a positional VDI identifier into a UUID.
func parseID(id string) (uuid.UUID, error) {
	u, err := uuid.FromString(id)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid VDI id %q (expected a UUID)", id)
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
	return cli.NotFound("VDI", "get", id, err, insecure)
}

// renderVDIRaw renders the VDI in the structured formats (json/yaml/text) or
// as a --query projection. It emits the raw object so machine readable output
// and the query pipeline are unchanged.
func renderVDIRaw(w io.Writer, format output.Format, vdi *payloads.VDI, query string) error {
	if query != "" {
		queryResult, err := output.Query(query, vdi)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, vdi, queryResult)
	}

	normalized, err := output.Normalize(vdi)
	if err != nil {
		return err
	}
	return output.Render(w, format, output.Table{}, normalized, nil)
}

// renderVDIDetail renders the single VDI as a human readable detail sheet.
// The SR is resolved by name and the attaching VMs are resolved in one batch
// (never one lookup per VM), a constant cost. A reference that cannot be
// resolved falls back to its raw id, so the view is always complete.
func renderVDIDetail(w io.Writer, ctx context.Context, xo library.Library, vdi *payloads.VDI, r *resolve.Client) error {
	lines := make([]string, 0, 12)

	// Header: type and name.
	header := "VDI " + vdi.NameLabel
	if vdi.Missing {
		header += "  (missing)"
	}
	lines = append(lines, header)

	// Identity
	if vdi.NameDescription != "" {
		lines = append(lines, output.DetailField("Description", vdi.NameDescription))
	}
	if len(vdi.Tags) > 0 {
		lines = append(lines, output.DetailField("Tags", strings.Join(vdi.Tags, ", ")))
	}

	// Location (resolved by name; the resolver falls back to the raw id when a
	// reference cannot be resolved, so the view is always complete).
	if name, err := r.SRName(ctx, vdi.SR); err == nil {
		lines = append(lines, output.DetailField("SR", name))
	} else {
		lines = append(lines, output.DetailField("SR", vdi.SR.String()))
	}

	// Resources
	lines = append(lines, output.DetailField("Type", string(vdi.VDIType)))
	lines = append(lines, output.DetailField("Size", sizeText(vdi.Size)))
	lines = append(lines, output.DetailField("Usage", sizeText(vdi.Usage)))
	if len(vdi.Snapshots) > 0 {
		lines = append(lines, output.DetailField("Snapshots", fmt.Sprintf("%d", len(vdi.Snapshots))))
	}
	if vdi.CBTEnabled != nil && *vdi.CBTEnabled {
		lines = append(lines, output.DetailField("CBT", "yes"))
	}

	// Relationships (the VMs the disk is attached to, resolved in one batch).
	if attached := attachedVMs(ctx, xo, vdi, r); attached != "" {
		lines = append(lines, output.DetailField("Attached to", attached))
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// attachedVMs returns the names of the VMs the VDI is attached to, resolved in
// one batch. It is best-effort: when the attaching VBDs cannot be fetched the
// line is simply omitted, so the detail view never fails because of it. The
// cost is constant (one VBD list filtered by the VDI, one VM name batch),
// never one lookup per VM.
func attachedVMs(ctx context.Context, xo library.Library, vdi *payloads.VDI, r *resolve.Client) string {
	if len(vdi.VBDs) == 0 {
		return ""
	}
	vbds, err := xo.VBD().GetAll(ctx, 0, "VDI:"+vdi.ID.String())
	if err != nil || len(vbds) == 0 {
		return ""
	}
	vmIDs := make([]uuid.UUID, 0, len(vbds))
	for _, v := range vbds {
		if !v.VM.IsNil() {
			vmIDs = append(vmIDs, v.VM)
		}
	}
	if len(vmIDs) == 0 {
		return ""
	}
	names := r.VMBatchNames(ctx, vmIDs)
	out := make([]string, 0, len(vmIDs))
	for _, id := range vmIDs {
		out = append(out, names[id.String()])
	}
	return strings.Join(out, ", ")
}

func sizeText(bytes int64) string {
	if bytes == 0 {
		return ""
	}
	return units.HumanSize(float64(bytes))
}
