// Package vdi implements the 'xo vdi' command group.
package vdi

import (
	"context"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/spf13/cobra"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

// NewCommand builds the 'xo vdi' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vdi",
		Short: "Manage virtual disks (VDIs)",
		Long: `Manage virtual disks (VDIs). A VDI is a disk that lives on a storage
repository (SR); it is not attached to a VM until a VBD links them (see
'xo vbd create').

VDIs are referenced by their UUID, as printed by 'xo vdi list'.`,
		Example: `  xo vdi list
  xo vdi create data --sr <sr-id> --size 20G
  xo vdi migrate <id> --sr <sr-id>`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newDeleteCommand())
	cmd.AddCommand(newMigrateCommand())
	cmd.AddCommand(newTagCommand())
	cmd.AddCommand(newExportCommand())
	cmd.AddCommand(newImportCommand())
	cli.CompleteIDs(cmd, listCandidates)
	return cmd
}

// listCandidates lists the virtual disks for the shell completion of their ids
// (see cli.CompleteIDs).
func listCandidates(ctx context.Context, xo library.Library) ([]cli.Candidate, error) {
	objects, err := xo.VDI().GetAll(ctx, 0, "")
	if err != nil {
		return nil, err
	}
	candidates := make([]cli.Candidate, 0, len(objects))
	for _, o := range objects {
		candidates = append(candidates, cli.Candidate{ID: o.ID.String(), Name: o.NameLabel})
	}
	return candidates, nil
}
