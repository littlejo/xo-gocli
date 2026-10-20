// Package network implements the 'xo network' command group.
package network

import (
	"context"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/spf13/cobra"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

// NewCommand builds the 'xo network' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage networks",
		Long: `List and inspect networks, create them (on a physical interface, internal
to the pool, or bonded over several interfaces), delete them, and manage
their tags.

Networks are referenced by their UUID, as printed by 'xo network list'.`,
		Example: `  xo network list
  xo network get <id>
  xo network create-internal backend --pool <pool-id>`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newTagCommand())
	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newCreateInternalCommand())
	cmd.AddCommand(newCreateBondedCommand())
	cmd.AddCommand(newDeleteCommand())
	cli.CompleteIDs(cmd, listCandidates)
	return cmd
}

// listCandidates lists the networks for the shell completion of their ids
// (see cli.CompleteIDs).
func listCandidates(ctx context.Context, xo library.Library) ([]cli.Candidate, error) {
	objects, err := xo.Network().GetAll(ctx, 0, "")
	if err != nil {
		return nil, err
	}
	candidates := make([]cli.Candidate, 0, len(objects))
	for _, o := range objects {
		candidates = append(candidates, cli.Candidate{ID: o.ID.String(), Name: o.NameLabel})
	}
	return candidates, nil
}
