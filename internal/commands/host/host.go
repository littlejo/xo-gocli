// Package host implements the 'xo host' command group.
package host

import (
	"github.com/spf13/cobra"
)

// NewCommand builds the 'xo host' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host",
		Short: "Manage hosts",
		Long: `List and inspect the hosts (XCP-ng or XenServer servers) that Xen
Orchestra manages, and manage their tags.

Hosts are referenced by their UUID, as printed by 'xo host list'.`,
		Example: `  xo host list
  xo host get <id>`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newTagCommand())
	return cmd
}
