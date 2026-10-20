// Package sr implements the 'xo sr' command group.
package sr

import (
	"context"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/spf13/cobra"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

// NewCommand builds the 'xo sr' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sr",
		Short: "Manage storage repositories",
		Long: `List and inspect storage repositories (SRs), the storage that holds the
virtual disks (VDIs); rescan them, reclaim their free space, and manage
their tags.

SRs are referenced by their UUID, as printed by 'xo sr list'.`,
		Example: `  xo sr list
  xo sr get <id>
  xo sr scan <id> --wait`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newScanCommand())
	cmd.AddCommand(newReclaimSpaceCommand())
	cmd.AddCommand(newTagCommand())
	cli.CompleteIDs(cmd, listCandidates)
	return cmd
}

// listCandidates lists the storage repositories for the shell completion of their ids
// (see cli.CompleteIDs).
func listCandidates(ctx context.Context, xo library.Library) ([]cli.Candidate, error) {
	objects, err := xo.SR().GetAll(ctx, 0, "")
	if err != nil {
		return nil, err
	}
	candidates := make([]cli.Candidate, 0, len(objects))
	for _, o := range objects {
		candidates = append(candidates, cli.Candidate{ID: o.ID.String(), Name: o.NameLabel})
	}
	return candidates, nil
}
