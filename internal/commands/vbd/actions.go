package vbd

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/taskwait"
)

func newConnectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect <id>",
		Short: "Hot-plug a VBD (attach the disk to a running VM)",
		Long: `Hot-plug a VBD, dynamically attaching its VDI to the running VM.

The VBD is referenced by its UUID, as returned by 'xo vbd list'. The VM must
be running for a hot-plug; if it is halted the disk is attached at boot
instead (no action needed).`,
		Example: `  xo vbd connect 33333333-3333-4333-8333-333333333333`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, args[0], "connect", func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
				return xo.VBD().Connect(ctx, id)
			})
		},
	}
	taskwait.AddFlag(cmd)
	return cmd
}

func newDisconnectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disconnect <id>",
		Short: "Hot-unplug a VBD (detach the disk from a running VM)",
		Long: `Hot-unplug a VBD, dynamically detaching its VDI from the running VM.

The VBD is referenced by its UUID, as returned by 'xo vbd list'. Use
'xo vbd delete <id>' instead to remove the attachment entirely (keeping the
VDI).`,
		Example: `  xo vbd disconnect 33333333-3333-4333-8333-333333333333`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, args[0], "disconnect", func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
				return xo.VBD().Disconnect(ctx, id)
			})
		},
	}
	taskwait.AddFlag(cmd)
	return cmd
}

// runAction implements the common flow shared by connect/disconnect: load the
// client, resolve the target VBD (as an existence check), run the action, and
// print the resulting task ID (or a plain confirmation when the action is
// synchronous).
func runAction(cmd *cobra.Command, idStr string, verb string, perform func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error)) error {
	xo, cfg, err := cli.LoadClient(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	id, err := parseID(idStr)
	if err != nil {
		return err
	}
	vbd, err := xo.VBD().Get(ctx, id)
	if err != nil {
		return cli.NotFound("VBD", "resolve", idStr, err, cfg.Insecure)
	}
	label := describe(vbd)

	taskID, err := perform(ctx, xo, id)
	if err != nil {
		return cli.InsecureHint(fmt.Sprintf("cannot %s VBD %s: %v", verb, label, err), cfg.Insecure)
	}

	if cli.WaitEnabled(cmd) && taskID != "" {
		return taskwait.ForCommand(ctx, cmd, cfg, taskID)
	}

	return taskwait.RenderStarted(cmd, "vbd", verb, label, label, taskID)
}
