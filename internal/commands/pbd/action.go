package pbd

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/taskwait"
)

// newPlugCommand builds 'xo pbd plug': connect the PBD, attaching the SR to
// its host (SDK PBD().Plug, asynchronous: returns a task id).
func newPlugCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plug <id>",
		Short: "Plug a PBD (connect the SR to its host)",
		Long: `Plug a PBD, connecting its storage repository (SR) to the host it belongs
to. Use 'xo pbd unplug <id>' to do the opposite.

The PBD is referenced by its UUID, as returned by 'xo pbd list'. The action
is asynchronous: the command prints the task id and returns. Track it with
'xo task get <task-id>' or 'xo task wait <task-id>'.`,
		Example: `  xo pbd plug 550e8400-e29b-41d4-a716-446655440001
  xo pbd plug <id> --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, args[0], "plug", func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
				return xo.PBD().Plug(ctx, id)
			})
		},
	}
	taskwait.AddFlag(cmd)
	return cmd
}

// newUnplugCommand builds 'xo pbd unplug': disconnect the PBD, detaching the
// SR from its host (SDK PBD().Unplug, asynchronous: returns a task id).
func newUnplugCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unplug <id>",
		Short: "Unplug a PBD (disconnect the SR from its host)",
		Long: `Unplug a PBD, disconnecting its storage repository (SR) from the host it
belongs to. Use 'xo pbd plug <id>' to re-connect it.

The PBD is referenced by its UUID, as returned by 'xo pbd list'. The action
is asynchronous: the command prints the task id and returns. Track it with
'xo task get <task-id>' or 'xo task wait <task-id>'.`,
		Example: `  xo pbd unplug 550e8400-e29b-41d4-a716-446655440001
  xo pbd unplug <id> --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, args[0], "unplug", func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
				return xo.PBD().Unplug(ctx, id)
			})
		},
	}
	taskwait.AddFlag(cmd)
	return cmd
}

// runAction implements the common flow shared by plug/unplug: load the
// client, resolve the target PBD (as an existence check), run the action, and
// print the resulting task ID.
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
	pbd, err := xo.PBD().Get(ctx, id)
	if err != nil {
		return cli.NotFound("PBD", "resolve", idStr, err, cfg.Insecure)
	}

	taskID, err := perform(ctx, xo, id)
	if err != nil {
		return cli.APIError(err, cfg.Insecure, "cannot %s PBD %q", verb, idStr)
	}

	if cli.WaitEnabled(cmd) && taskID != "" {
		return taskwait.ForCommand(ctx, cmd, cfg, taskID)
	}

	label := describe(pbd)
	return taskwait.RenderStarted(cmd, "pbd", verb, label, label, taskID)
}

// describe renders a PBD for messages: the PBD has no name_label, so the label
// links its host and SR.
func describe(pbd *payloads.PBD) string {
	return fmt.Sprintf("PBD %s (host %s, SR %s)", pbd.ID.String(), pbd.Host.String(), pbd.SR.String())
}
