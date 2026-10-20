package vm

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/littlejo/xo-gocli/internal/taskwait"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

func newRebootCommand() *cobra.Command {
	var hard bool

	cmd := &cobra.Command{
		Use:   "reboot <id>",
		Short: "Reboot a virtual machine",
		Long: `Reboot a virtual machine.

By default this performs a clean reboot (the guest is asked to restart).
Use --hard to force a hard reboot without a clean shutdown.

The VM is referenced by its UUID, as returned by 'xo vm list'.`,
		Example: `  xo vm reboot 550e8400-e29b-41d4-a716-446655440001
  xo vm reboot <id> --hard`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			verb := "reboot"
			if hard {
				verb = "hard reboot"
			}
			return runAction(cmd, actionSpec{
				verb: verb,
				id:   args[0],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
					if hard {
						return xo.VM().HardReboot(ctx, id)
					}
					return xo.VM().CleanReboot(ctx, id)
				},
			})
		},
	}

	cmd.Flags().BoolVar(&hard, flagHard, false, "force a hard reboot instead of a clean one")
	taskwait.AddFlag(cmd)
	return cmd
}
