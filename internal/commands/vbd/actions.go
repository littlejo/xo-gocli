package vbd

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/taskwait"
)

// flagWait makes an asynchronous action block until its task completes and
// render the task, instead of returning right after the action is started.
const flagWait = "wait"

// addWaitFlag registers the --wait flag on an asynchronous action. The
// $XOA_WAIT environment variable is an equivalent for scripts (cli.WaitEnabled).
func addWaitFlag(cmd *cobra.Command) {
	cmd.Flags().Bool(flagWait, false, "wait for the task to complete before returning, and print it (like 'xo task wait'; or set XOA_WAIT=1)")
}

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
	addWaitFlag(cmd)
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
	addWaitFlag(cmd)
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
		return waitOnTask(cmd, cfg, ctx, taskID)
	}

	return renderActionResult(cmd, verb, label, taskID)
}

// waitOnTask blocks until the action's task reaches a terminal state and
// renders the completed task (like 'xo task wait'); the returned error
// reflects the outcome so the exit status does too. Each poll is bounded by
// the HTTP client timeout (--timeout / $XOA_TIMEOUT); the wait itself ends on
// a terminal state or Ctrl+C.
func waitOnTask(cmd *cobra.Command, cfg *config.ClientConfig, ctx context.Context, taskID string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	httpClient, err := cli.NewHTTPClient(cmd, cfg)
	if err != nil {
		return err
	}
	return taskwait.Wait(ctx, httpClient, taskwait.Options{
		Out:    cmd.OutOrStdout(),
		Stderr: cmd.ErrOrStderr(),
		ID:     taskID,
		Format: format,
		NotFound: func(id string, err error) error {
			return cli.NotFound("task", "get", id, err, cfg.Insecure)
		},
	})
}

// renderActionResult prints the outcome of an async VBD action: a friendly
// line for the human formats and a small machine document for json/yaml. When
// the action was synchronous (empty task id), no task id is printed.
func renderActionResult(cmd *cobra.Command, verb, label, taskID string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		doc := map[string]any{"action": verb, "vbd": label}
		if taskID != "" {
			doc["task_id"] = taskID
		}
		raw, err := output.Normalize(doc)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		if taskID == "" {
			_, err := fmt.Fprintf(w, "Requested %s of %s\n", verb, label)
			return err
		}
		_, err := fmt.Fprintf(w, "Requested %s of %s (task %s)\n", verb, label, taskID)
		return err
	}
}
