package vbd

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
	flagLimit = "limit"
	flagVM    = "vm"
)

func newListCommand() *cobra.Command {
	var (
		query string
		limit int
		vmID  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List virtual block devices (VBDs)",
		Long: `List virtual block devices (VBDs) from Xen Orchestra.

A VBD is the attachment point between a VM and a VDI (virtual disk).

Examples:
  xo vbd list
  xo vbd list --vm <vm-id>
  xo vbd list --output json
  xo vbd list --query '[].VDI'
  xo vbd list --limit 10`,
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
			filter := ""
			if vmID != "" {
				vm, err := parseID(vmID)
				if err != nil {
					return err
				}
				filter = "VM:" + vm.String()
			}

			cfg, err := config.Load(cli.ProfileName(cmd))
			if err != nil {
				return err
			}
			xo, err := cli.NewClient(cmd, cfg)
			if err != nil {
				return err
			}

			vbds, err := xo.VBD().GetAll(cmd.Context(), limit, filter)
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot list VBDs: %v", err), cfg.Insecure)
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
			return renderVBDs(cmd.OutOrStdout(), cmd.Context(), format, vbds, query, resolver)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. '[].VDI'")
	flags.IntVar(&limit, flagLimit, 0, "maximum number of VBDs to return (0 for no limit)")
	flags.StringVar(&vmID, flagVM, "", "only list the VBDs of the given VM (UUID)")

	return cmd
}

// renderVBDs applies the optional --query expression and renders the result
// in the requested format. The human table shows the attached VM and VDI by
// name (at most two batch lookups for every row, never one lookup per VBD);
// the structured formats and --query still emit the raw objects, unchanged.
func renderVBDs(w io.Writer, ctx context.Context, format output.Format, vbds []*payloads.VBD, query string, r *resolve.Client) error {
	queryResult, err := output.Query(query, vbds)
	if err != nil {
		return err
	}

	var vms, vdis []uuid.UUID
	for _, vbd := range vbds {
		if !vbd.VM.IsNil() {
			vms = append(vms, vbd.VM)
		}
		if vbd.VDI != nil && !vbd.VDI.IsNil() {
			vdis = append(vdis, *vbd.VDI)
		}
	}
	vmNames := map[string]string{}
	if r != nil && len(vms) > 0 {
		vmNames = r.VMBatchNames(ctx, vms)
	}
	vdiNames := map[string]string{}
	if r != nil && len(vdis) > 0 {
		vdiNames = r.VDIBatchNames(ctx, vdis)
	}

	table := output.Table{
		Headers: []string{"ID", "VM", "VDI", "DEVICE", "MODE", "ATTACHED"},
	}
	for _, vbd := range vbds {
		vm := vbd.VM.String()
		if n, ok := vmNames[vbd.VM.String()]; ok {
			vm = n
		}
		vdi := vdiText(vbd.VDI)
		if vbd.VDI != nil {
			if n, ok := vdiNames[vbd.VDI.String()]; ok {
				vdi = n
			}
		}
		table.Rows = append(table.Rows, []string{
			vbd.ID.String(),
			vm,
			vdi,
			deviceText(vbd.Device),
			modeText(vbd),
			boolText(vbd.Attached),
		})
	}

	// For structured formats without a query, normalize the SDK types so the
	// output only contains the requested data.
	var raw any = vbds
	if format != output.FormatTable && (queryResult == nil || !queryResult.Present) {
		normalized, err := output.Normalize(vbds)
		if err != nil {
			return err
		}
		raw = normalized
	}

	return output.Render(w, format, table, raw, queryResult)
}
