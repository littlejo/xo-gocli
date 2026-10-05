package vm

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
	flagQuery      = "query"
	flagLimit      = "limit"
	flagPowerState = "power-state"
)

func newListCommand() *cobra.Command {
	var (
		query      string
		limit      int
		powerState string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List virtual machines",
		Long: `List virtual machines from Xen Orchestra.

Examples:
  xo vm list
  xo vm list --output json
  xo vm list --query '[].name_label'
  xo vm list --power-state Running
  xo vm list --limit 10`,
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
			filter := powerState
			if filter != "" {
				filter = "power_state:" + powerState
			}

			cfg, err := config.Load(cli.ProfileName(cmd))
			if err != nil {
				return err
			}

			xo, err := cli.NewClient(cmd, cfg)
			if err != nil {
				return err
			}

			vms, err := xo.VM().GetAll(cmd.Context(), limit, filter)
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot list VMs: %v", err), cfg.Insecure)
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
			return renderVMs(cmd.OutOrStdout(), cmd.Context(), format, vms, query, resolver)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. '[].name_label'")
	flags.IntVar(&limit, flagLimit, 0, "maximum number of VMs to return (0 for no limit)")
	flags.StringVar(&powerState, flagPowerState, "", "filter by power state: Running, Halted, Paused, Suspended")

	return cmd
}

// renderVMs applies the optional --query expression and renders the result in
// the requested format. The human table shows the container (the host the VM
// runs on, or the pool it belongs to in pool mode) by name instead of by UUID;
// the names come from at most two batch lookups (all hosts, all pools), never
// one lookup per VM. The structured formats and --query still emit the raw
// objects, unchanged.
func renderVMs(w io.Writer, ctx context.Context, format output.Format, vms []*payloads.VM, query string, r *resolve.Client) error {
	queryResult, err := output.Query(query, vms)
	if err != nil {
		return err
	}

	// The container is a pool in pool mode, or a host when the VM is pinned to
	// one; the VM's $poolId field is what disambiguates the two. Resolving
	// both collections at once costs two batch lookups whatever the list size,
	// and an unresolvable id falls back to its raw string.
	var containers []uuid.UUID
	for _, vm := range vms {
		if !vm.Container.IsNil() {
			containers = append(containers, vm.Container)
		}
	}
	var hostNames, poolNames map[string]string
	if r != nil && len(containers) > 0 {
		hostNames = r.HostBatchNames(ctx, containers)
		poolNames = r.PoolBatchNames(ctx, containers)
	}

	table := output.Table{
		Headers: []string{"ID", "NAME", "POWER STATE", "MEMORY", "CPUS", "IP", "CONTAINER"},
		Empty:   "VMs",
	}
	for _, vm := range vms {
		name := vm.Container.String()
		if vm.PoolID == vm.Container {
			if n, ok := poolNames[vm.Container.String()]; ok {
				name = n
			}
		} else if n, ok := hostNames[vm.Container.String()]; ok {
			name = n
		}
		table.Rows = append(table.Rows, []string{
			vm.ID.String(),
			vm.NameLabel,
			vm.PowerState,
			memoryText(vm),
			fmt.Sprintf("%d", vm.CPUs.Number),
			output.OrDash(vm.MainIpAddress),
			name,
		})
	}

	// For structured formats without a query, normalize the SDK types so the
	// output only contains the requested data.
	var raw any = vms
	if format != output.FormatTable && (queryResult == nil || !queryResult.Present) {
		normalized, err := output.Normalize(vms)
		if err != nil {
			return err
		}
		raw = normalized
	}

	return output.Render(w, format, table, raw, queryResult)
}

func memoryText(vm *payloads.VM) string {
	return units.HumanSize(float64(vm.Memory.Size))
}
