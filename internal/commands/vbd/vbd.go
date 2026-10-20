// Package vbd implements the 'xo vbd' command group.
package vbd

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// NewCommand builds the 'xo vbd' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vbd",
		Short: "Manage virtual block devices (VBDs)",
		Long: `Manage virtual block devices (VBDs). A VBD is the attachment between a VM
and a VDI: creating one plugs a disk into a VM, deleting it unplugs the
disk without deleting the VDI.

VBDs are referenced by their UUID, as printed by 'xo vbd list'.`,
		Example: `  xo vbd list
  xo vbd create --vm <vm-id> --vdi <vdi-id>
  xo vbd connect <id> --wait`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newDeleteCommand())
	cmd.AddCommand(newConnectCommand())
	cmd.AddCommand(newDisconnectCommand())
	cli.CompleteIDs(cmd, listCandidates)
	return cmd
}

// parseID converts a positional VBD identifier into a UUID.
func parseID(id string) (uuid.UUID, error) {
	return cli.ParseID("VBD", id)
}

// notFound delegates to cli.NotFound, which reports a 404 concisely and keeps
// the raw API error as debug detail.
func notFound(id string, err error, insecure bool) error {
	return cli.NotFound("VBD", "get", id, err, insecure)
}

// listCandidates lists the VBDs for the shell completion of their ids
// (see cli.CompleteIDs).
func listCandidates(ctx context.Context, xo library.Library) ([]cli.Candidate, error) {
	objects, err := xo.VBD().GetAll(ctx, 0, "")
	if err != nil {
		return nil, err
	}
	candidates := make([]cli.Candidate, 0, len(objects))
	for _, o := range objects {
		candidates = append(candidates, cli.Candidate{ID: o.ID.String(), Name: vbdDevice(o)})
	}
	return candidates, nil
}

// vbdDevice is the device name shown beside a VBD id in completions ("xvda"),
// empty while the VBD is not plugged.
func vbdDevice(o *payloads.VBD) string {
	if o.Device == nil {
		return ""
	}
	return *o.Device
}
