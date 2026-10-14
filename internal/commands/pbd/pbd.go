// Package pbd implements the 'xo pbd' command group.
package pbd

import (
	"github.com/spf13/cobra"
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
	return cmd
}
