package sr

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

func newReclaimSpaceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reclaim-space <id>",
		Short: "Reclaim unused space from a storage repository",
		Long: `Reclaim unused space from a storage repository.

Space reclamation (thin-provisioning "unmap") returns unused disk space to
the underlying storage. It only applies to SR types that support it (for
example thin-provisioned LVM or VDI-based stores); other SR types will be
rejected by Xen Orchestra. The operation is asynchronous and returns a task
id.

The SR is referenced by its UUID, as returned by 'xo sr list'.`,
		Example: `  xo sr reclaim-space 550e8400-e29b-41d4-a716-446655440001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, actionSpec{
				verb: "reclaim space",
				id:   args[0],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
					return xo.SR().ReclaimSpace(ctx, id)
				},
			})
		},
	}
	addWaitFlag(cmd)
	return cmd
}
