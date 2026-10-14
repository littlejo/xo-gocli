package vm

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

func newSuspendCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suspend <id>",
		Short: "Suspend a virtual machine",
		Long: `Suspend a virtual machine.

Suspending saves the VM's memory to its storage and releases it: the VM is
in "Suspended" state until resumed with 'xo vm resume <id>'. The VM must be
running. This is the memory-preserving alternative to 'xo vm stop':
suspending keeps the guest's state exactly, whereas stopping shuts it down.

The VM is referenced by its UUID, as returned by 'xo vm list'.`,
		Example: `  xo vm suspend 550e8400-e29b-41d4-a716-446655440001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, actionSpec{
				verb: "suspend",
				id:   args[0],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
					return xo.VM().Suspend(ctx, id)
				},
			})
		},
	}
	addWaitFlag(cmd)
	return cmd
}
