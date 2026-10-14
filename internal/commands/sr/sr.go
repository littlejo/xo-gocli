// Package sr implements the 'xo sr' command group.
package sr

import (
	"github.com/spf13/cobra"
)

// NewCommand builds the 'xo sr' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sr",
		Short: "Manage storage repositories",
		Long: `List and inspect storage repositories (SRs), the storage that holds the
virtual disks (VDIs); rescan them, reclaim their free space, and manage
their tags.

SRs are referenced by their UUID, as printed by 'xo sr list'.`,
		Example: `  xo sr list
  xo sr get <id>
  xo sr scan <id> --wait`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newScanCommand())
	cmd.AddCommand(newReclaimSpaceCommand())
	cmd.AddCommand(newTagCommand())
	return cmd
}
