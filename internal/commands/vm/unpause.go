package vm

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

func newUnpauseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unpause <id>",
		Short: "Unpause a virtual machine",
		Long: `Unpause a virtual machine.

The VM's vCPUs resume where 'xo vm pause' stopped them. The VM is
referenced by its UUID, as returned by 'xo vm list'.`,
		Example: `  xo vm unpause 550e8400-e29b-41d4-a716-446655440001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, actionSpec{
				verb: "unpause",
				id:   args[0],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
					return xo.VM().Unpause(ctx, id)
				},
			})
		},
	}
	addWaitFlag(cmd)
	return cmd
}
