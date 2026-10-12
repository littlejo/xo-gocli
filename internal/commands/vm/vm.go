// Package vm implements the 'xo vm' command group.
package vm

import (
	"github.com/spf13/cobra"
)

// NewCommand builds the 'xo vm' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vm",
		Short: "Manage virtual machines",
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
	cmd.AddCommand(newWaitCommand())
	cmd.AddCommand(newVdisCommand())
	cmd.AddCommand(newExportCommand())
	cmd.AddCommand(newImportCommand())
	cmd.AddCommand(newDeleteCommand())
	return cmd
}
