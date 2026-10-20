package vm

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/littlejo/xo-gocli/internal/taskwait"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

func newPauseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pause <id>",
		Short: "Pause a virtual machine",
		Long: `Pause a virtual machine.

Pausing suspends the execution of all the VM's vCPUs: the VM keeps its RAM
contents and its power state reads "Paused" while paused. Resume it with
'xo vm unpause <id>'. Pausing is not the same as suspending: a suspended VM
is saved to disk and its memory is released (see 'xo vm suspend').

The VM is referenced by its UUID, as returned by 'xo vm list'.`,
		Example: `  xo vm pause 550e8400-e29b-41d4-a716-446655440001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, actionSpec{
				verb: "pause",
				id:   args[0],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
					return xo.VM().Pause(ctx, id)
				},
			})
		},
	}
	taskwait.AddFlag(cmd)
	return cmd
}
