package sr

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

func newScanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan <id>",
		Short: "Scan a storage repository",
		Long: `Scan a storage repository.

Scanning re-reads the underlying storage so Xen Orchestra picks up new and
removed disks (VDIs) on the SR. The operation is asynchronous and returns a
task id; the new state becomes visible once the scan task has completed.

The SR is referenced by its UUID, as returned by 'xo sr list'.`,
		Example: `  xo sr scan 550e8400-e29b-41d4-a716-446655440001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, actionSpec{
				verb: "scan",
				id:   args[0],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
					return xo.SR().Scan(ctx, id)
				},
			})
		},
	}
	addWaitFlag(cmd)
	return cmd
}
