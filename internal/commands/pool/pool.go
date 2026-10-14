// Package pool implements the 'xo pool' command group.
package pool

import (
	"github.com/spf13/cobra"
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
	return cmd
}
