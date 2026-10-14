package task

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
	v2client "github.com/vatesfr/xenorchestra-go-sdk/v2/client"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

// flagYes skips the interactive confirmation for destructive operations. It is
// required whenever stdin is not a terminal so automation never blocks; the
// $XOA_YES environment variable is an equivalent for scripts (cli.SkipConfirm).
const flagYes = "yes"

func newAbortCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "abort <id>",
		Short: "Abort a running task",
		Long: `Abort a task that is still running.

Xen Orchestra requests the interruption of the operation backing the task
(a VM start, a migration, a provisioning, ...). The task then reaches the
"interrupted" status; the operation itself may take a little time to unwind,
and it is not always fully reversible (for example, a VM start aborted
mid-way leaves the VM in the state it reached).

Only a task that is still pending can be aborted: aborting a task that
already reached a terminal state (success, failure or interrupted) is
rejected with a clear error.

This is a destructive operation and asks for confirmation unless --yes is
given. The task is referenced by its id, as returned by 'xo task list' or by
an asynchronous operation (for example 'xo vm start').`,
		Example: `  xo task abort a1b2c3d4e5f6
  xo task abort <id> --yes
  xo task wait <id> --wait-timeout 5m   # then abort if it is stuck`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if id == "" || strings.Contains(id, "/") {
				return fmt.Errorf("invalid task id %q", id)
			}
			return runAbort(cmd, id, cli.SkipConfirm(cmd))
		},
	}
	// --yes (or $XOA_YES) skips the confirmation; it is read via
	// cli.SkipConfirm, which sees both the flag and the environment variable.
	cmd.Flags().Bool(flagYes, false, "do not ask for confirmation (or set XOA_YES=1)")
	return cmd
}

// newClients loads the selected profile and builds the two SDK v2 clients the
// abort flow needs: the typed library (Task().Abort) and the exported REST
// client (the existence pre-check, for the same reason 'xo task get' uses it
// rather than the typed Get — see internal/taskwait). Commands must pass
// their cobra context to the SDK operations so that cancellation (Ctrl+C)
// reaches the HTTP layer.
func newClients(cmd *cobra.Command) (library.Library, *v2client.Client, *config.ClientConfig, error) {
	cfg, err := config.Load(cli.ProfileName(cmd))
	if err != nil {
		return nil, nil, nil, err
	}
	xo, err := cli.NewClient(cmd, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	httpClient, err := cli.NewHTTPClient(cmd, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	return xo, httpClient, cfg, nil
}

// taskLabel renders the task for human messages: its id plus, when available,
// the operation it is running (e.g. "f6e5d4c3b2a1 (VM clean_shutdown)").
func taskLabel(t map[string]any) string {
	id := strField(t, "id")
	props, _ := t["properties"].(map[string]any)
	ptype, pname := strField(props, "type"), strField(props, "name")
	var desc string
	switch {
	case ptype != "" && pname != "":
		desc = ptype + " " + pname
	case ptype != "":
		desc = ptype
	default:
		desc = pname
	}
	if desc != "" {
		return fmt.Sprintf("%s (%s)", id, desc)
	}
	return id
}

// runAbort implements the abort flow: load the clients, fetch the task (as an
// existence and status pre-check), confirm, abort, and print a concise
// confirmation.
func runAbort(cmd *cobra.Command, id string, yes bool) error {
	xo, httpClient, cfg, err := newClients(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	// The pre-check goes through the SDK's own REST client, the same single
	// API boundary as 'xo task get': the typed Task().Get unmarshals into
	// payloads.Task, whose Result is a struct, but XO sometimes returns a
	// task "result" as a plain string (a documented XO quirk), so the typed
	// Get fails on such tasks.
	var current map[string]any
	if err := v2client.TypedGet(ctx, httpClient, tasksEndpoint+"/"+id, listParams{Fields: "*"}, &current); err != nil {
		return taskNotFound(id, err, cfg.Insecure)
	}

	status := strField(current, "status")
	if status != "pending" {
		return fmt.Errorf("task %q cannot be aborted: it is already %s", id, status)
	}

	label := taskLabel(current)
	ok, err := confirm(cmd, fmt.Sprintf("Are you sure you want to abort task %s?", label), yes)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}

	if err := xo.Task().Abort(ctx, id); err != nil {
		return cli.InsecureHint(fmt.Sprintf("cannot abort task %q: %v", id, err), cfg.Insecure)
	}

	return renderAborted(cmd, id)
}

// renderAborted prints the outcome of a task abort: a friendly line for the
// human formats and a small machine document for json/yaml.
func renderAborted(cmd *cobra.Command, id string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"action": "abort", "task": id})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Aborted task %s\n", id)
		return err
	}
}

// isTerminal reports whether in is connected to a terminal.
func isTerminal(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

// confirm asks the user to confirm a destructive operation. When --yes is
// provided it always proceeds; when stdin is not a terminal it refuses so that
// automation never blocks, forcing the caller to pass --yes explicitly.
func confirm(cmd *cobra.Command, message string, yes bool) (bool, error) {
	if yes {
		return true, nil
	}
	in := cmd.InOrStdin()
	if !isTerminal(in) {
		return false, fmt.Errorf("confirmation required: re-run with --yes to proceed non-interactively")
	}
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N]: ", message); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	if err != nil && answer == "" {
		return false, fmt.Errorf("cannot read confirmation: %w", err)
	}
	return answer == "y" || answer == "yes", nil
}
