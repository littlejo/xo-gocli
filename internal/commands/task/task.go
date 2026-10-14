// Package task implements the 'xo task' command group.
package task

import (
	"github.com/spf13/cobra"
)

// NewCommand builds the 'xo task' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage asynchronous tasks",
		Long: `List and inspect the asynchronous tasks Xen Orchestra runs (VM actions,
migrations, imports, ...), wait for one to complete, or abort it.

'xo task wait' exits 0 only when the task succeeds, so it can gate a
script.

Tasks are referenced by their id, as printed by 'xo task list' or by an
asynchronous command.`,
		Example: `  xo task list --status pending
  xo task wait <id> --wait-timeout 10m
  xo task abort <id>`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newWaitCommand())
	cmd.AddCommand(newAbortCommand())
	return cmd
}
