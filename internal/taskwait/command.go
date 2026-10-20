package taskwait

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

// FlagWait makes an asynchronous action block until its task completes and
// render the task, instead of returning right after the action is started.
const FlagWait = "wait"

// AddFlag registers the --wait flag on an asynchronous action. The $XOA_WAIT
// environment variable is an equivalent for scripts (cli.WaitEnabled).
func AddFlag(cmd *cobra.Command) {
	cmd.Flags().Bool(FlagWait, false, "wait for the task to complete before returning, and print it (like 'xo task wait'; or set XOA_WAIT=1)")
}

// ForCommand waits for the task an action started, the way 'xo task wait'
// does: it renders the task in the command's --output format and fails when
// the task does not succeed.
func ForCommand(ctx context.Context, cmd *cobra.Command, cfg *config.ClientConfig, taskID string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	httpClient, err := cli.NewHTTPClient(cmd, cfg)
	if err != nil {
		return err
	}
	return Wait(ctx, httpClient, Options{
		Out:    cmd.OutOrStdout(),
		Stderr: cmd.ErrOrStderr(),
		ID:     taskID,
		Format: format,
		NotFound: func(id string, err error) error {
			return cli.NotFound("task", "get", id, err, cfg.Insecure)
		},
	})
}

// RenderStarted prints the outcome of an asynchronous action that was started
// without --wait: a friendly line for the human formats ("Requested <verb> of
// <label> (task <id>)") and a small machine document for json/yaml, whose key
// kind ("vm", "sr", ...) holds target. A synchronous action (empty task id)
// prints no task id.
func RenderStarted(cmd *cobra.Command, kind, verb, target, label, taskID string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		doc := map[string]any{"action": verb, kind: target}
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
