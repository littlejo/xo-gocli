package vm

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

const flagHost = "host"

func newStartCommand() *cobra.Command {
	var hostID string

	cmd := &cobra.Command{
		Use:   "start <id>",
		Short: "Start a virtual machine",
		Long: `Start a virtual machine.

The VM is referenced by its UUID, as returned by 'xo vm list'. Use --host to
pin the VM to a specific host; otherwise Xen Orchestra selects one.`,
		Example: `  xo vm start 550e8400-e29b-41d4-a716-446655440001
  xo vm start <id> --host 6b7c8d9e-0000-1111-2222-333344445555`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var target *uuid.UUID
			if hostID != "" {
				h, err := uuid.FromString(hostID)
				if err != nil {
					return fmt.Errorf("invalid --host id %q (expected a UUID)", hostID)
				}
				target = &h
			}

			return runAction(cmd, actionSpec{
				verb: "start",
				id:   args[0],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
					return xo.VM().Start(ctx, id, target)
				},
			})
		},
	}

	cmd.Flags().StringVar(&hostID, flagHost, "", "host UUID to start the VM on (default: automatic)")
	addWaitFlag(cmd)
	return cmd
}
