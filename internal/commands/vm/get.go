package vm

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

func newGetCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a virtual machine",
		Long: `Get a virtual machine from Xen Orchestra, as a detailed, human
readable view.

The VM is referenced by its UUID, as returned by 'xo vm list'.

The human (default) view is a detail sheet: identity, location (the
host or pool the VM runs on, shown by name), resources and configuration.
The relationships are resolved to names by a small number of extra lookups
(the container and the template); if one cannot be resolved, the raw id is
shown instead and the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo vm get <id> --output json' is unchanged.

Examples:
  xo vm get 550e8400-e29b-41d4-a716-446655440001
  xo vm get <id> --output json
  xo vm get <id> --query 'name_label'`,
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

			vm, err := xo.VM().GetByID(cmd.Context(), id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}

			// The structured formats (json/yaml/text) and --query are served
			// straight from the raw object, exactly as before; only the default
			// human view is the new detail sheet, which resolves relationships.
			if format != output.FormatTable || query != "" {
				return renderVMRaw(cmd.OutOrStdout(), format, vm, query)
			}

			resolver, err := cli.NewResolver(cmd, cfg)
			if err != nil {
				return err
			}
			return renderVMDetail(cmd.OutOrStdout(), cmd.Context(), vm, resolver)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'name_label'")
	return cmd
}

// renderVMRaw renders the VM in the structured formats (json/yaml/text) or as
// a --query projection. It emits the raw object so machine readable output and
// the query pipeline are unchanged.
func renderVMRaw(w io.Writer, format output.Format, vm *payloads.VM, query string) error {
	if query != "" {
		queryResult, err := output.Query(query, vm)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, vm, queryResult)
	}

	normalized, err := output.Normalize(vm)
	if err != nil {
		return err
	}
	return output.Render(w, format, output.Table{}, normalized, nil)
}

// renderVMDetail renders the single VM as a human readable detail sheet. The
// relationships (container, template) are resolved to names by the resolver;
// a reference that cannot be resolved falls back to its raw id, so the view
// is always complete. The cost is a constant number of extra lookups (at most
// one host and one pool for the container, one template), independent of the
// pool size.
func renderVMDetail(w io.Writer, ctx context.Context, vm *payloads.VM, r *resolve.Client) error {
	lines := make([]string, 0, 24)

	// Header: type, name and (power state) — the one-line identity.
	header := "VM " + vm.NameLabel
	if vm.PowerState != "" {
		header += "  (" + vm.PowerState + ")"
	}
	lines = append(lines, header)

	// Identity
	if vm.MainIpAddress != "" {
		lines = append(lines, output.DetailField("IP", vm.MainIpAddress))
	}
	if vm.NameDescription != "" {
		lines = append(lines, output.DetailField("Description", vm.NameDescription))
	}
	if len(vm.Tags) > 0 {
		lines = append(lines, output.DetailField("Tags", strings.Join(vm.Tags, ", ")))
	}

	// Location (resolved by name; the helper falls back to the raw id when a
	// reference cannot be resolved, so the view is always complete).
	if name, err := r.ResolveContainer(ctx, vm.Container); err == nil {
		lines = append(lines, output.DetailField("Container", name))
	} else {
		lines = append(lines, output.DetailField("Container", vm.Container.String()))
	}
	// The template's REST id is composite ("poolId-templateUuid"); it is built
	// from the VM's own pool and template uuids. If it cannot be resolved the
	// raw composite id is shown, which is what 'xo template get' accepts.
	if !vm.Template.IsNil() && !vm.PoolID.IsNil() {
		name, _ := r.Template(ctx, templateID(vm))
		lines = append(lines, output.DetailField("Template", output.OrDash(name)))
	}

	// Resources
	lines = append(lines, output.DetailField("Memory", output.OrDash(memoryText(vm))))
	lines = append(lines, output.DetailField("CPUs", cpuText(vm)))
	if len(vm.VBDs) > 0 {
		lines = append(lines, output.DetailField("Disks", fmt.Sprintf("%d  (xo vm vdis %s)", len(vm.VBDs), vm.ID)))
	}
	if len(vm.VIFs) > 0 {
		lines = append(lines, output.DetailField("Networks", fmt.Sprintf("%d", len(vm.VIFs))))
	}
	if len(vm.Snapshots) > 0 {
		lines = append(lines, output.DetailField("Snapshots", fmt.Sprintf("%d", len(vm.Snapshots))))
	}

	// Configuration
	lines = append(lines, output.DetailField("Boot", bootText(vm)))
	flags := vmFlags(vm)
	lines = append(lines, output.DetailField("Flags", output.OrDash(flags)))

	// Status
	if len(vm.BlockedOperations) > 0 {
		keys := make([]string, 0, len(vm.BlockedOperations))
		for k, v := range vm.BlockedOperations {
			if v != "" {
				keys = append(keys, string(k)+"="+v)
			} else {
				keys = append(keys, string(k))
			}
		}
		sortStrings(keys)
		lines = append(lines, output.DetailField("Blocked", strings.Join(keys, ", ")))
	}
	if d := createdText(vm.Creation); d != "" {
		lines = append(lines, output.DetailField("Created", d))
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// templateID builds the composite VM template REST id ("poolId-templateUuid")
// from the VM's own pool and template uuids. It is the id form 'xo template
// get' and the vm-templates endpoint use.
func templateID(vm *payloads.VM) string {
	return vm.PoolID.String() + "-" + vm.Template.String()
}

// cpuText renders "n" or "n (max m)" when a cap is set.
func cpuText(vm *payloads.VM) string {
	if vm.CPUs.Max > 0 && vm.CPUs.Max != vm.CPUs.Number {
		return fmt.Sprintf("%d (max %d)", vm.CPUs.Number, vm.CPUs.Max)
	}
	return fmt.Sprintf("%d", vm.CPUs.Number)
}

// bootText renders the boot firmware and order, omitting empty parts.
func bootText(vm *payloads.VM) string {
	var parts []string
	if vm.Boot.Firmware != "" {
		parts = append(parts, vm.Boot.Firmware)
	}
	if vm.Boot.Order != "" {
		parts = append(parts, "order "+vm.Boot.Order)
	}
	return output.OrDash(strings.Join(parts, ", "))
}

// vmFlags renders the non-default configuration as a compact, space separated
// list of human readable tags; empty when everything is at its default.
func vmFlags(vm *payloads.VM) string {
	var f []string
	if vm.VirtualizationMode != "" {
		f = append(f, vm.VirtualizationMode)
	}
	if vm.AutoPoweron {
		f = append(f, "auto-poweron")
	}
	if vm.ExpNestedHvm {
		f = append(f, "nested-hvm")
	}
	if vm.HA != "" {
		f = append(f, "HA="+vm.HA)
	}
	if vm.Videoram != 0 {
		f = append(f, fmt.Sprintf("videoram=%d", vm.Videoram))
	}
	if vm.Vga != "" && vm.Vga != "std" {
		f = append(f, "vga="+vm.Vga)
	}
	return strings.Join(f, " ")
}

// createdText renders "date by user" from the VM's creation info, or "" when
// unknown. The API date is not pinned to one format, so several are tried
// before falling back to the raw value.
func createdText(c payloads.Creation) string {
	parts := make([]string, 0, 2)
	if c.Date != "" {
		parts = append(parts, formatAPIDate(c.Date))
	}
	if c.User != "" {
		parts = append(parts, "by "+c.User)
	}
	return strings.Join(parts, " ")
}

// formatAPIDate normalizes an API date to a stable UTC form, or returns it as
// is when it does not match a known layout.
func formatAPIDate(s string) string {
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format("2006-01-02T15:04:05Z")
		}
	}
	return s
}

// sortStrings is a small insertion sort, kept local so the detail renderer has
// no hidden dependency on an output helper.
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
