package pool

import (
	"context"
	"time"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

func newRollingUpdateCommand() *cobra.Command {
	var waitTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "rolling-update <id>",
		Short: "Apply the latest pool update, rolling through the hosts",
		Long: `Apply the latest available pool update.

The update (new Xen Orchestra agent / tools versions) is rolled across the
hosts of the pool one by one, so the pool stays available the whole time.
Each host is rebooted in turn with the VMs that run on it moved away first
(live migration when possible, otherwise a scheduled downtime for the VMs).

The command is synchronous: it blocks until the backing task completes (or
Ctrl+C), and reports the final result. There is no default wait deadline —
a pool update can legitimately run for a long time — so use --timeout to
bound the wait if you need one. Note that this --timeout is the *wait*
deadline, not the global HTTP client timeout; it shadows the global flag on
this command (like 'xo task wait').

The pool is referenced by its UUID, as returned by 'xo pool list'.

Examples:
  xo pool rolling-update aaaaaaaa-bbbb-cccc-dddd-000000000001
  xo pool rolling-update <id> --timeout 30m
  xo pool rolling-update <id> --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, args[0], actionSpec{
				verb: "rolling update",
				past: "rolling update applied",
				do: func(ctx context.Context, xo library.Library, id uuid.UUID) error {
					return xo.Pool().RollingUpdate(ctx, id)
				},
			})
		},
	}
	addWaitTimeoutFlag(cmd, &waitTimeout)
	return cmd
}
