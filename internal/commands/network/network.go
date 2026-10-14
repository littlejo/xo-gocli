// Package network implements the 'xo network' command group.
package network

import (
	"github.com/spf13/cobra"
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
	return cmd
}
