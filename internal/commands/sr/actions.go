package sr

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

// nameOf resolves the name_label of the SR with the given id. It also acts as
// an existence check so actions fail with a clear "not found" error before the
// operation is performed.
func nameOf(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
	sr, err := xo.SR().Get(ctx, id)
	if err != nil {
		return "", err
	}
	if sr.NameLabel == "" {
		return id.String(), nil
	}
	return sr.NameLabel, nil
}

// actionSpec describes an async SR action so each command only supplies what
// differs: the verb shown to the user and the SDK call to run.
type actionSpec struct {
	verb    string
	id      string
	perform func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error)
}

// runAction implements the common flow shared by the SR async actions: load
// the client, resolve the target SR, run the action, and print the resulting
// task ID.
func runAction(cmd *cobra.Command, spec actionSpec) error {
	xo, cfg, err := cli.LoadClient(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	id, err := parseID(spec.id)
	if err != nil {
		return err
	}
	name, err := nameOf(ctx, xo, id)
	if err != nil {
		return cli.NotFound("SR", "resolve", spec.id, err, cfg.Insecure)
	}

	taskID, err := spec.perform(ctx, xo, id)
	if err != nil {
		return cli.InsecureHint(fmt.Sprintf("cannot %s SR %q: %v", spec.verb, name, err), cfg.Insecure)
	}

	if cli.WaitEnabled(cmd) && taskID != "" {
		return waitOnTask(cmd, cfg, ctx, taskID)
	}

	return renderActionResult(cmd, spec.verb, name, taskID)
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

// renderActionResult prints the outcome of an async SR action: a friendly line
// for the human formats and a small machine document for json/yaml.
func renderActionResult(cmd *cobra.Command, verb, name, taskID string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"action": verb, "sr": name, "task_id": taskID})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Requested %s of %q (task %s)\n", verb, name, taskID)
		return err
	}
}
