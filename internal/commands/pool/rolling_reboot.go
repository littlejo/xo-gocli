package pool

import (
	"context"
	"time"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

func newRollingRebootCommand() *cobra.Command {
	var waitTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "rolling-reboot <id>",
		Short: "Reboot the pool's hosts one by one",
		Long: `Reboot all the hosts of a pool, one at a time.

The hosts are rebooted in turn: the VMs that run on the host being rebooted
are moved away first (live migration when possible), the host is rebooted,
and it rejoins the pool before the next one goes down. The pool stays
available the whole time, but VMs that cannot be live-migrated away (e.g.
without an available target) suffer a brief downtime on their host's turn.

This is a destructive operation and asks for confirmation unless --yes is
given. The command is synchronous: it blocks until the backing task completes
(or Ctrl+C) and reports the final result. There is no default wait deadline,
so use --timeout to bound the wait if you need one; on this command that
--timeout is the *wait* deadline (not the global HTTP client timeout) and it
shadows the global flag.

The pool is referenced by its UUID, as returned by 'xo pool list'.

Examples:
  xo pool rolling-reboot aaaaaaaa-bbbb-cccc-dddd-000000000001
  xo pool rolling-reboot <id> --yes
  xo pool rolling-reboot <id> --yes --timeout 30m`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAction(cmd, args[0], actionSpec{
				verb:        "rolling reboot",
				past:        "rolling reboot done",
				destructive: true,
				do: func(ctx context.Context, xo library.Library, id uuid.UUID) error {
					return xo.Pool().RollingReboot(ctx, id)
				},
			})
		},
	}
	cmd.Flags().Bool(flagYes, false, "do not ask for confirmation (or set XOA_YES=1)")
	addWaitTimeoutFlag(cmd, &waitTimeout)
	return cmd
}
