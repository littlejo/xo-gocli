// Package pbd implements the 'xo pbd' command group.
package pbd

import (
	"context"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/spf13/cobra"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

// NewCommand builds the 'xo pbd' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pbd",
		Short: "Manage physical block devices (PBDs)",
		Long: `Manage physical block devices (PBDs). A PBD is the connection between a
host and a storage repository: plugging it makes the SR available on that
host, unplugging it disconnects the SR from the host.

PBDs are referenced by their UUID, as printed by 'xo pbd list'.`,
		Example: `  xo pbd list
  xo pbd unplug <id> --wait`,
	}
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newPlugCommand())
	cmd.AddCommand(newUnplugCommand())
	cli.CompleteIDs(cmd, listCandidates)
	return cmd
}

// listCandidates lists the PBDs for the shell completion of their ids
// (see cli.CompleteIDs).
func listCandidates(ctx context.Context, xo library.Library) ([]cli.Candidate, error) {
	objects, err := xo.PBD().GetAll(ctx, 0, "")
	if err != nil {
		return nil, err
	}
	candidates := make([]cli.Candidate, 0, len(objects))
	for _, o := range objects {
		candidates = append(candidates, cli.Candidate{ID: o.ID.String(), Name: ""})
	}
	return candidates, nil
}
