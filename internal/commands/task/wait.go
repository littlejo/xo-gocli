package task

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/taskwait"
)

func newWaitCommand() *cobra.Command {
	var (
		query       string
		waitTimeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "wait <id>",
		Short: "Wait for a task to complete",
		Long: `Wait for a task to reach a terminal state (success, failure or
interrupted), polling every 2 seconds, and print it like 'xo task get'.

The command blocks until the task completes. Ctrl+C cancels the wait. Use
--wait-timeout to bound how long to wait; without it the wait is unbounded (it ends
only when the task completes or is interrupted).

Exit status reflects the task's outcome, so it is safe to use as a gate in
scripts:
  - 0 when the task completes successfully
  - non-zero when the task fails, is interrupted, the --wait-timeout deadline
    is reached, or the task does not exist
The completed task is always printed (in the chosen --output format); a
non-zero exit is reported on stderr.

The global --timeout (or $XOA_TIMEOUT) still bounds each HTTP request, that
is each poll, not the whole wait.

The task is referenced by its id, as returned by 'xo task list' or by an
asynchronous operation (for example 'xo vm start').`,
		Example: `  xo task wait a1b2c3d4e5f6
  xo task wait <id> --wait-timeout 5m
  xo task wait <id> --output json
  xo task wait <id> --query 'status'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if id == "" || strings.Contains(id, "/") {
				return fmt.Errorf("invalid task id %q", id)
			}
			if err := output.ValidateQuery(query); err != nil {
				return err
			}
			if waitTimeout < 0 {
				return fmt.Errorf("--wait-timeout must be greater than or equal to 0")
			}
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}

			cfg, err := config.Load(cli.ProfileName(cmd))
			if err != nil {
				return err
			}

			httpClient, err := cli.NewHTTPClient(cmd, cfg)
			if err != nil {
				return err
			}

			// The wait is bounded by --wait-timeout when given; otherwise it runs
			// until the task completes or the command context is cancelled
			// (Ctrl+C propagates from the CLI down to the HTTP layer). The
			// polling and rendering are shared with the --wait flag of the
			// asynchronous actions (internal/taskwait).
			return taskwait.Wait(cmd.Context(), httpClient, taskwait.Options{
				Out:      cmd.OutOrStdout(),
				Stderr:   cmd.ErrOrStderr(),
				ID:       id,
				Format:   format,
				Query:    query,
				Deadline: waitTimeout,
				NotFound: func(id string, err error) error {
					return taskNotFound(id, err, cfg.Insecure)
				},
			})
		},
	}
	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the completed task, e.g. 'status'")
	// The wait deadline; the global --timeout stays the per-request one.
	cmd.Flags().DurationVar(&waitTimeout, cli.FlagWaitTimeout, 0, "wait at most this long for the task to complete, e.g. 2m (default: wait until it completes)")

	return cmd
}
