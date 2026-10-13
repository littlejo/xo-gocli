package network

import (
	"context"
	"fmt"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagPool     = "pool"
	flagPif      = "pif"
	flagPifs     = "pifs"
	flagVlan     = "vlan"
	flagMTU      = "mtu"
	flagNBD      = "nbd"
	flagBondMode = "bond-mode"
)

// bondModes are the bonding modes accepted by the create_bonded_network
// pool action, in the order shown in the usage.
var bondModes = []string{
	string(payloads.NetworkBondModeActiveBackup),
	string(payloads.NetworkBondModeBalanceSLB),
	string(payloads.NetworkBondModeLACP),
}

func newCreateCommand() *cobra.Command {
	var (
		poolID      string
		pifID       string
		description string
		vlan        uint
		mtu         int
		nbd         bool
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a network in a pool",
		Long: `Create a network in a pool, attached to a PIF (physical interface) of
one of the pool's hosts.

The pool is referenced by its UUID, as returned by 'xo pool list'. The PIF
is referenced by its UUID as well; PIFs can be listed with 'xo rest get pifs'
(there is no typed PIF command yet).

The creation is asynchronous server-side; this command waits for the backing
task to finish before printing the created network. The wait has no
deadline: it ends when the task completes, or on Ctrl+C.

Examples:
  xo network create web --pool <pool-id> --pif <pif-id>
  xo network create vlan-100 --pool <pool-id> --pif <pif-id> --vlan 100
  xo network create mgmt --pool <pool-id> --pif <pif-id> --mtu 9000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if mtu < 0 {
				return fmt.Errorf("--mtu must be greater than or equal to 0")
			}
			pool, err := parsePoolID(poolID)
			if err != nil {
				return err
			}
			pif, err := parsePifID(pifID)
			if err != nil {
				return err
			}
			return runCreate(cmd, args[0], func(ctx context.Context, xo library.Library) (uuid.UUID, error) {
				return xo.Network().Create(ctx, pool, payloads.CreateNetworkParams{
					Name:        args[0],
					Description: description,
					Pif:         pif,
					Vlan:        vlan,
					MTU:         optionalInt(mtu),
					NBD:         optionalBool(nbd),
				})
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&poolID, flagPool, "", "pool UUID to create the network in (required, see 'xo pool list')")
	flags.StringVar(&pifID, flagPif, "", "PIF (physical interface) UUID to attach the network to (required, see 'xo rest get pifs')")
	flags.UintVar(&vlan, flagVlan, 0, "VLAN tag, between 0 and 4094 (0 for an untagged network)")
	flags.IntVar(&mtu, flagMTU, 0, "network MTU (0 for the XenServer default, 1500)")
	flags.StringVar(&description, "description", "", "description for the network")
	flags.BoolVar(&nbd, flagNBD, false, "allow NBD access to the network")

	return cmd
}

func newCreateInternalCommand() *cobra.Command {
	var (
		poolID      string
		description string
		mtu         int
		nbd         bool
	)

	cmd := &cobra.Command{
		Use:   "create-internal <name>",
		Short: "Create an internal network in a pool",
		Long: `Create an internal network in a pool.

An internal network does not attach to a physical interface; it carries
virtual traffic between VMs within the pool (for example a management or a
dedicated VM-to-VM network).

The pool is referenced by its UUID, as returned by 'xo pool list'. The
creation is asynchronous server-side; this command waits for the backing
task to finish before printing the created network. The wait has no
deadline: it ends when the task completes, or on Ctrl+C.

Examples:
  xo network create-internal internal --pool <pool-id>
  xo network create-internal internal --pool <pool-id> --mtu 9000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if mtu < 0 {
				return fmt.Errorf("--mtu must be greater than or equal to 0")
			}
			pool, err := parsePoolID(poolID)
			if err != nil {
				return err
			}
			return runCreate(cmd, args[0], func(ctx context.Context, xo library.Library) (uuid.UUID, error) {
				return xo.Network().CreateInternal(ctx, pool, payloads.CreateInternalNetworkParams{
					Name:        args[0],
					Description: description,
					MTU:         optionalInt(mtu),
					NBD:         optionalBool(nbd),
				})
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&poolID, flagPool, "", "pool UUID to create the network in (required, see 'xo pool list')")
	flags.IntVar(&mtu, flagMTU, 0, "network MTU (0 for the XenServer default, 1500)")
	flags.StringVar(&description, "description", "", "description for the network")
	flags.BoolVar(&nbd, flagNBD, false, "allow NBD access to the network")

	return cmd
}

func newCreateBondedCommand() *cobra.Command {
	var (
		poolID      string
		pifs        string
		bondMode    string
		description string
		mtu         int
		nbd         bool
	)

	cmd := &cobra.Command{
		Use:   "create-bonded <name>",
		Short: "Create a bonded network in a pool",
		Long: `Create a bonded network in a pool.

A bonded network links several PIFs (physical interfaces) of the pool hosts
into a single logical network for redundancy or aggregation.

The pool is referenced by its UUID, as returned by 'xo pool list'; the PIFs
by their UUIDs, as listed by 'xo rest get pifs'. --bond-mode selects the
bonding mode: active-backup, balance-slb or lacp.

The creation is asynchronous server-side; this command waits for the backing
task to finish before printing the created network. The wait has no
deadline: it ends when the task completes, or on Ctrl+C.

Examples:
  xo network create-bonded bond0 --pool <pool-id> --pifs <pif-1>,<pif-2> --bond-mode active-backup
  xo network create-bonded bond0 --pool <pool-id> --pifs <pif-1>,<pif-2> --bond-mode lacp --mtu 9000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if mtu < 0 {
				return fmt.Errorf("--mtu must be greater than or equal to 0")
			}
			pool, err := parsePoolID(poolID)
			if err != nil {
				return err
			}
			pifIDs, err := parsePifs(pifs)
			if err != nil {
				return err
			}
			mode := payloads.NetworkBondMode(strings.ToLower(strings.TrimSpace(bondMode)))
			if mode == "" {
				return fmt.Errorf("--bond-mode is required (one of %s)", strings.Join(bondModes, ", "))
			}
			switch mode {
			case payloads.NetworkBondModeActiveBackup, payloads.NetworkBondModeBalanceSLB, payloads.NetworkBondModeLACP:
			default:
				return fmt.Errorf("invalid --bond-mode %q (expected one of %s)", bondMode, strings.Join(bondModes, ", "))
			}
			return runCreate(cmd, args[0], func(ctx context.Context, xo library.Library) (uuid.UUID, error) {
				return xo.Network().CreateBonded(ctx, pool, payloads.CreateBondedNetworkParams{
					Name:        args[0],
					Description: description,
					MTU:         optionalInt(mtu),
					NBD:         optionalBool(nbd),
					PifIds:      pifIDs,
					BondMode:    mode,
				})
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&poolID, flagPool, "", "pool UUID to create the network in (required, see 'xo pool list')")
	flags.StringVar(&pifs, flagPifs, "", "comma-separated PIF UUIDs to bond (required, see 'xo rest get pifs')")
	flags.StringVar(&bondMode, flagBondMode, "", "bonding mode: active-backup, balance-slb or lacp (required)")
	flags.IntVar(&mtu, flagMTU, 0, "network MTU (0 for the XenServer default, 1500)")
	flags.StringVar(&description, "description", "", "description for the network")
	flags.BoolVar(&nbd, flagNBD, false, "allow NBD access to the network")

	return cmd
}

// parsePoolID converts the --pool flag into a UUID.
func parsePoolID(value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.Nil, fmt.Errorf("--pool is required (see 'xo pool list')")
	}
	u, err := uuid.FromString(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid --pool id %q (expected a UUID)", value)
	}
	return u, nil
}

// parsePifID converts the --pif flag into a UUID.
func parsePifID(value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.Nil, fmt.Errorf("--pif is required (see 'xo rest get pifs')")
	}
	u, err := uuid.FromString(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid --pif id %q (expected a UUID)", value)
	}
	return u, nil
}

// parsePifs converts the comma-separated --pifs flag into a list of UUIDs.
func parsePifs(value string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0)
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		u, err := uuid.FromString(part)
		if err != nil {
			return nil, fmt.Errorf("invalid --pifs entry %q (expected a UUID)", part)
		}
		ids = append(ids, u)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("--pifs must contain at least one PIF UUID")
	}
	return ids, nil
}

// optionalInt returns a pointer to v, or nil when v is zero so that optional
// fields keep the server default.
func optionalInt(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

// optionalBool returns a pointer to b, or nil when b is false so that the
// optional field is omitted.
func optionalBool(b bool) *bool {
	if !b {
		return nil
	}
	return &b
}

// runCreate implements the common flow shared by the three create commands:
// load the client, run the given SDK create operation (which posts the pool
// action, waits for the backing task and returns the created network id),
// re-fetch the network, and print the result.
func runCreate(cmd *cobra.Command, name string, create func(ctx context.Context, xo library.Library) (uuid.UUID, error)) error {
	xo, cfg, err := newClient(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	networkID, err := create(ctx, xo)
	if err != nil {
		return cli.InsecureHint(fmt.Sprintf("cannot create network %q: %v", name, err), cfg.Insecure)
	}

	// The create task returned the network id; re-fetch the network so the
	// output carries its real fields (bridge, MTU, ...).
	network, err := xo.Network().Get(ctx, networkID)
	if err != nil {
		// The network was created; a transient fetch failure should only
		// cost the extra fields, not the result itself.
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: network created but could not be re-fetched: %v\n", err)
		network = nil
	}

	return renderCreatedNetwork(cmd, name, networkID, network)
}

// renderCreatedNetwork prints the outcome of a network create: the re-fetched
// network for the structured formats, a friendly summary for the human
// format. When the re-fetch failed, only the id is known.
func renderCreatedNetwork(cmd *cobra.Command, name string, networkID uuid.UUID, network *payloads.Network) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		var data any = network
		if network == nil {
			data = map[string]any{"action": "create", "network": name, "id": networkID.String()}
		}
		normalized, err := output.Normalize(data)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, normalized, nil)
	default:
		if network == nil {
			_, err := fmt.Fprintf(w, "Network %q created (id %s)\n", name, networkID)
			return err
		}
		_, err := fmt.Fprintf(w, "Network %q created:\n  id:     %s\n  mtu:    %d\n  bridge: %s\n\nList it with: xo network list\n",
			name, network.ID.String(), network.MTU, network.Bridge)
		return err
	}
}
