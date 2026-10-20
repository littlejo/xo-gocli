// Package pool implements the 'xo pool' command group.
package pool

import (
	"context"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/spf13/cobra"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

// NewCommand builds the 'xo pool' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage pools",
		Long: `List and inspect pools, manage their tags, and run the pool-wide
maintenance operations: rolling update, rolling reboot and emergency
shutdown. The maintenance commands are synchronous: they block until the
operation completes (bound the wait with --wait-timeout).

Pools are referenced by their UUID, as printed by 'xo pool list'.`,
		Example: `  xo pool list
  xo pool get <id>
  xo pool rolling-update <id> --wait-timeout 2h`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newTagCommand())
	cmd.AddCommand(newRollingUpdateCommand())
	cmd.AddCommand(newRollingRebootCommand())
	cmd.AddCommand(newEmergencyShutdownCommand())
	cli.CompleteIDs(cmd, listCandidates)
	return cmd
}

// listCandidates lists the pools for the shell completion of their ids
// (see cli.CompleteIDs).
func listCandidates(ctx context.Context, xo library.Library) ([]cli.Candidate, error) {
	objects, err := xo.Pool().GetAll(ctx, 0, "")
	if err != nil {
		return nil, err
	}
	candidates := make([]cli.Candidate, 0, len(objects))
	for _, o := range objects {
		candidates = append(candidates, cli.Candidate{ID: o.ID.String(), Name: o.NameLabel})
	}
	return candidates, nil
}
