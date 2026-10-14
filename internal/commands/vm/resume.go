package vm

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

func newResumeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume <id>",
		Short: "Resume a suspended virtual machine",
		Long: `Resume a virtual machine that was suspended with 'xo vm suspend'.

The VM's saved memory is restored and the guest continues where it left
off. Unlike 'xo vm start', the VM must be in the "Suspended" state — a
halted VM has no saved memory to restore.

The VM is referenced by its UUID, as returned by 'xo vm list'.`,
		Example: `  xo vm resume 550e8400-e29b-41d4-a716-446655440001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, actionSpec{
				verb: "resume",
				id:   args[0],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
					return xo.VM().Resume(ctx, id)
				},
			})
		},
	}
	addWaitFlag(cmd)
	return cmd
}
