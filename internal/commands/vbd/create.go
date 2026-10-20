package vbd

import (
	"fmt"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagVDI      = "vdi"
	flagMode     = "mode"
	flagBootable = "bootable"
)

// vbdModes are the access modes a VBD can take.
var vbdModes = []string{string(payloads.VBDModeRO), string(payloads.VBDModeRW)}

func newCreateCommand() *cobra.Command {
	var (
		vmID     string
		vdiID    string
		mode     string
		bootable bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Attach a VDI to a VM",
		Long: `Attach a virtual disk (VDI) to a virtual machine (VM) by creating a VBD.

The VM and the VDI are referenced by their UUID, as returned by
'xo vm list' and 'xo vdi list' (or 'xo vm vdis <id>') respectively. The VDI
must live on an SR that is shared by the pool the VM belongs to, or on a
local SR of the host the VM runs on.

Creating the VBD does not hot-plug the disk: if the VM is running, hot-plug
it afterwards with 'xo vbd connect <id>' (or reboot the VM). Detach it with
'xo vbd delete <id>' (this only removes the attachment; the VDI itself is
kept).`,
		Example: `  xo vbd create --vm <vm-id> --vdi <vdi-id>
  xo vbd create --vm <vm-id> --vdi <vdi-id> --mode RO
  xo vbd create --vm <vm-id> --vdi <vdi-id> --bootable`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if vmID == "" {
				return fmt.Errorf("--vm is required (see 'xo vm list')")
			}
			if vdiID == "" {
				return fmt.Errorf("--vdi is required (see 'xo vdi list')")
			}
			vm, err := parseID(vmID)
			if err != nil {
				return fmt.Errorf("invalid --vm id %q (expected a UUID)", vmID)
			}
			vdi, err := parseID(vdiID)
			if err != nil {
				return fmt.Errorf("invalid --vdi id %q (expected a UUID)", vdiID)
			}
			m := payloads.VBDMode(mode)
			if m != "" {
				switch m {
				case payloads.VBDModeRO, payloads.VBDModeRW:
				default:
					return fmt.Errorf("invalid --mode %q (expected one of %s)", mode, strings.Join(vbdModes, ", "))
				}
			}

			params := &payloads.CreateVBDParams{
				VM:   vm,
				VDI:  vdi,
				Mode: m,
				Type: payloads.VBDTypeDisk,
			}
			if bootable {
				b := true
				params.Bootable = &b
			}

			xo, cfg, err := cli.LoadClient(cmd)
			if err != nil {
				return err
			}
			vbdID, err := xo.VBD().Create(cmd.Context(), params)
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot attach VDI %q to VM %q: %v", vdiID, vmID, err), cfg.Insecure)
			}

			return renderCreated(cmd, vmID, vdiID, vbdID)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&vmID, flagVM, "", "VM UUID to attach the VDI to (required, see 'xo vm list')")
	flags.StringVar(&vdiID, flagVDI, "", "VDI UUID to attach (required, see 'xo vdi list')")
	flags.StringVar(&mode, flagMode, "", "access mode: RO or RW (default RW)")
	flags.BoolVar(&bootable, flagBootable, false, "make the VBD a boot device")

	return cmd
}

// renderCreated prints the outcome of a VBD create: the id for every format,
// plus a hint toward the next step (hot-plug for a running VM).
func renderCreated(cmd *cobra.Command, vmID, vdiID string, vbdID uuid.UUID) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"action": "create", "vm": vmID, "vdi": vdiID, "vbd": vbdID.String()})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "VBD created (id %s): VDI %s attached to VM %s\n\nIf the VM is running, hot-plug the disk with: xo vbd connect %s\n",
			vbdID, vdiID, vmID, vbdID)
		return err
	}
}
