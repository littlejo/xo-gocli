package vbd

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
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

const flagQuery = "query"

func newGetCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a virtual block device (VBD)",
		Long: `Get a virtual block device (VBD) from Xen Orchestra, as a detailed,
human readable view.

A VBD is the attachment point between a VM and a VDI (virtual disk). The
VBD is referenced by its UUID, as returned by 'xo vbd list'.

The human (default) view is a detail sheet that resolves the relationships
by name: the VM the VBD belongs to, and the VDI it points at (with its
size). If a reference cannot be resolved, the raw id is shown instead and
the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo vbd get <id> --output json' is unchanged.

Examples:
  xo vbd get 33333333-3333-4333-8333-333333333333
  xo vbd get <id> --output json
  xo vbd get <id> --query 'VDI'`,
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

			vbd, err := xo.VBD().Get(cmd.Context(), id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}

			// The structured formats (json/yaml/text) and --query are served
			// straight from the raw object, exactly as before; only the default
			// human view is the new detail sheet, which resolves relationships.
			if format != output.FormatTable || query != "" {
				return renderVBDRaw(cmd.OutOrStdout(), format, vbd, query)
			}

			resolver, err := cli.NewResolver(cmd, cfg)
			if err != nil {
				return err
			}
			return renderVBDDetail(cmd.OutOrStdout(), cmd.Context(), xo, vbd, resolver)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'VDI'")
	return cmd
}

// renderVBDRaw renders the VBD in the structured formats (json/yaml/text) or
// as a --query projection. It emits the raw object so machine readable output
// and the query pipeline are unchanged.
func renderVBDRaw(w io.Writer, format output.Format, vbd *payloads.VBD, query string) error {
	if query != "" {
		queryResult, err := output.Query(query, vbd)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, vbd, queryResult)
	}

	normalized, err := output.Normalize(vbd)
	if err != nil {
		return err
	}
	return output.Render(w, format, output.Table{}, normalized, nil)
}

// renderVBDDetail renders the single VBD as a human readable detail sheet.
// The two relationships (the VM and the VDI) are resolved to names at a
// constant cost (one VM name lookup and one VDI lookup). A reference that
// cannot be resolved falls back to its raw id, so the view is always complete.
func renderVBDDetail(w io.Writer, ctx context.Context, xo library.Library, vbd *payloads.VBD, r *resolve.Client) error {
	lines := make([]string, 0, 10)

	// Header: type, device and (mode, attachment).
	header := "VBD " + deviceText(vbd.Device)
	header += fmt.Sprintf("  (%s, attached=%s)", modeText(vbd), boolText(vbd.Attached))
	if vbd.Bootable {
		header += ", bootable"
	}
	lines = append(lines, header)

	// Identity
	if vbd.IsCDDrive {
		lines = append(lines, output.DetailField("Type", "CD drive"))
	}
	if vbd.Device != nil && *vbd.Device != "" {
		lines = append(lines, output.DetailField("Device", *vbd.Device))
	}
	if vbd.Position != 0 {
		lines = append(lines, output.DetailField("Position", fmt.Sprintf("%d", vbd.Position)))
	}

	// Relationships (resolved by name; the resolver falls back to the raw id
	// when a reference cannot be resolved, so the view is always complete).
	if name, err := r.VMName(ctx, vbd.VM); err == nil {
		lines = append(lines, output.DetailField("VM", name))
	} else {
		lines = append(lines, output.DetailField("VM", vbd.VM.String()))
	}
	if vdiRef := vbd.VDI; vdiRef != nil && !vdiRef.IsNil() {
		name, size := vdiLabel(ctx, xo, *vdiRef)
		lines = append(lines, output.DetailField("VDI", output.OrDash(name)))
		if size != "" {
			lines = append(lines, output.DetailField("Size", size))
		}
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// vdiLabel resolves the name and size of the VDI a VBD points at. The VDI is
// fetched once (a constant lookup) so both its name and its virtual size are
// available; the raw id is returned when it cannot be resolved.
func vdiLabel(ctx context.Context, xo library.Library, vdi uuid.UUID) (string, string) {
	d, err := xo.VDI().Get(ctx, vdi)
	if err != nil || d == nil {
		return vdi.String(), ""
	}
	size := ""
	if d.Size > 0 {
		size = units.HumanSize(float64(d.Size))
	}
	return d.NameLabel, size
}

func vdiText(vdi *uuid.UUID) string {
	if vdi == nil || *vdi == uuid.Nil {
		return "-"
	}
	return vdi.String()
}

func deviceText(device *string) string {
	if device == nil || *device == "" {
		return "-"
	}
	return *device
}

func modeText(vbd *payloads.VBD) string {
	if vbd.ReadOnly {
		return "RO"
	}
	return "RW"
}

func boolText(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
