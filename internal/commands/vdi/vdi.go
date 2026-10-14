// Package vdi implements the 'xo vdi' command group.
package vdi

import (
	"github.com/spf13/cobra"
)

// NewCommand builds the 'xo vdi' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vdi",
		Short: "Manage virtual disks (VDIs)",
		Long: `Manage virtual disks (VDIs). A VDI is a disk that lives on a storage
repository (SR); it is not attached to a VM until a VBD links them (see
'xo vbd create').

VDIs are referenced by their UUID, as printed by 'xo vdi list'.`,
		Example: `  xo vdi list
  xo vdi create data --sr <sr-id> --size 20G
  xo vdi migrate <id> --sr <sr-id>`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newDeleteCommand())
	cmd.AddCommand(newMigrateCommand())
	cmd.AddCommand(newTagCommand())
	cmd.AddCommand(newExportCommand())
	cmd.AddCommand(newImportCommand())
	return cmd
}
