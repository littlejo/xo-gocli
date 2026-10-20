// Package vm implements the 'xo vm' command group.
package vm

import (
	"context"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/spf13/cobra"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

// NewCommand builds the 'xo vm' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vm",
		Short: "Manage virtual machines",
		Long: `Manage virtual machines: list and inspect them, create, update and delete
them, drive their power state, snapshot, export and import them, and manage
their tags.

Lifecycle actions (start, stop, reboot, pause, suspend, ...) are
asynchronous: they print a task id and return. Add --wait (or set
$XOA_WAIT) to block until the task completes.

VMs are referenced by their UUID, as printed by 'xo vm list'.`,
		Example: `  xo vm list
  xo vm list --query "[?power_state=='Running'].name_label"
  xo vm get <id>
  xo vm start <id> --wait`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newUpdateCommand())
	cmd.AddCommand(newTagCommand())
	cmd.AddCommand(newStartCommand())
	cmd.AddCommand(newStopCommand())
	cmd.AddCommand(newRebootCommand())
	cmd.AddCommand(newPauseCommand())
	cmd.AddCommand(newUnpauseCommand())
	cmd.AddCommand(newSuspendCommand())
	cmd.AddCommand(newResumeCommand())
	cmd.AddCommand(newSnapshotCommand())
	cmd.AddCommand(newVdisCommand())
	cmd.AddCommand(newExportCommand())
	cmd.AddCommand(newImportCommand())
	cmd.AddCommand(newDeleteCommand())
	cli.CompleteIDs(cmd, listCandidates)
	return cmd
}

// listCandidates lists the virtual machines for the shell completion of their ids
// (see cli.CompleteIDs).
func listCandidates(ctx context.Context, xo library.Library) ([]cli.Candidate, error) {
	objects, err := xo.VM().GetAll(ctx, 0, "")
	if err != nil {
		return nil, err
	}
	candidates := make([]cli.Candidate, 0, len(objects))
	for _, o := range objects {
		candidates = append(candidates, cli.Candidate{ID: o.ID.String(), Name: o.NameLabel})
	}
	return candidates, nil
}
