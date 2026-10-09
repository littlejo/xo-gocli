package vm

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/taskwait"
)

const (
	flagWaitSSH  = "ssh"
	flagWaitPort = "port"

	// defaultWaitPort is the port probed by --ssh when --port is not given.
	defaultWaitPort = 22

	// sshProbeTimeout bounds a single reachability probe to the guest. A slow
	// or refusing answer means "not ready yet", not an error: the guest may
	// still be starting sshd or opening its firewall.
	sshProbeTimeout = 5 * time.Second
)

func newWaitCommand() *cobra.Command {
	var (
		query       string
		waitTimeout time.Duration
		sshGate     bool
		port        int
	)

	cmd := &cobra.Command{
		Use:   "wait <id>",
		Short: "Wait until a VM is ready to use",
		Long: `Wait until a VM is ready to use, polling every 2 seconds (like
'xo task wait').

By default the command waits until the VM is Running and has a main IP
address (the 'IP' column of 'xo vm list'). With --ssh it also waits until the
guest's SSH port is reachable over TCP on that IP, so 'xo vm wait <id>
--ssh' is the last gate before logging in. The probe checks TCP
reachability only, not the SSH handshake: if the guest starts sshd late or
opens its firewall late, the port may answer only a moment later.

The command blocks until the gate is satisfied. Ctrl+C cancels the wait. Use
--timeout to bound how long to wait; without it the wait is unbounded (it
ends only when the gate is satisfied or you press Ctrl+C).

Exit status: 0 when the VM is ready; non-zero when the --timeout deadline is
reached, the wait is interrupted, or the VM does not exist.

Note: this command's --timeout bounds the *wait* only (how long to keep
polling). It does not change the per-request HTTP timeout: each poll still
uses the global --timeout / $XOA_TIMEOUT / 30s default. To raise the HTTP
timeout applied to each poll, set the global --timeout or $XOA_TIMEOUT —
extending the wait alone does not.

The VM is referenced by its UUID, as returned by 'xo vm list'.

Examples:
  xo vm wait 550e8400-e29b-41d4-a716-446655440001
  xo vm wait <id> --ssh                  # …until the guest answers on :22
  xo vm wait <id> --ssh --port 2222      # non-standard port
  xo vm wait <id> --timeout 5m           # give up after 5 minutes
  ip=$(xo vm wait <id> --ssh --output json --query ip)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if waitTimeout < 0 {
				return fmt.Errorf("--timeout must be greater than or equal to 0")
			}
			if port <= 0 || port > 65535 {
				return fmt.Errorf("invalid --port %d (expected a port between 1 and 65535)", port)
			}
			if err := output.ValidateQuery(query); err != nil {
				return err
			}
			// --port is only meaningful for the ssh gate: passing it implies
			// --ssh.
			if cmd.Flags().Changed(flagWaitPort) {
				sshGate = true
			}
			return runWait(cmd, args[0], query, waitTimeout, vmWaitGate{ssh: sshGate, port: port})
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'ip'")
	// Local --timeout: the wait deadline (how long to keep polling). It is a
	// distinct local flag that happens to reuse the global name "timeout"; it
	// does NOT affect the per-request HTTP timeout (cli.Timeout reads the
	// root's persistent flag, which stays at its own value) — see the Long
	// help.
	cmd.Flags().DurationVar(&waitTimeout, cli.FlagTimeout, 0, "wait at most this long for the VM to be ready, e.g. 5m (default: wait until it is ready); bounds the wait only, not the HTTP timeout of each poll")
	cmd.Flags().BoolVar(&sshGate, flagWaitSSH, false, "also wait until the guest's SSH port is reachable on its main IP (TCP probe)")
	cmd.Flags().IntVar(&port, flagWaitPort, defaultWaitPort, "port to probe with --ssh (default 22; implies --ssh)")
	return cmd
}

// vmWaitGate describes what "ready" means for a vm wait: at minimum the VM
// must be Running with a main IP; the ssh gate additionally requires the port
// to be reachable on that IP.
type vmWaitGate struct {
	ssh  bool
	port int
}

// check reports what is still missing for the VM to be ready, or "" when it
// is ready. A port that is not (yet) reachable is a blocker, not an error:
// the guest may still be starting sshd.
func (g vmWaitGate) check(ctx context.Context, vm *payloads.VM) string {
	state := vmState(vm)
	if state != "Running" {
		return fmt.Sprintf("state is %s", state)
	}
	if vm.MainIpAddress == "" {
		return "no main IP address yet"
	}
	if !g.ssh {
		return ""
	}
	addr := net.JoinHostPort(vm.MainIpAddress, strconv.Itoa(g.port))
	dialer := net.Dialer{Timeout: sshProbeTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Sprintf("port %d on %s is not reachable yet", g.port, vm.MainIpAddress)
	}
	_ = conn.Close()
	return ""
}

// runWait polls the VM (SDK v2 GetByID) until the gate is satisfied, then
// renders the readiness document. Each poll is bounded by the HTTP client
// timeout; the wait itself is bounded by --timeout when given and always by
// the command context (Ctrl+C propagates from the CLI through the SDK to the
// HTTP layer).
func runWait(cmd *cobra.Command, idArg, query string, waitTimeout time.Duration, gate vmWaitGate) error {
	xo, cfg, err := newClient(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if waitTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, waitTimeout)
		defer cancel()
	}

	id, err := parseID(idArg)
	if err != nil {
		return err
	}
	name, err := nameOf(ctx, xo, id)
	if err != nil {
		return notFound(idArg, err, cfg.Insecure)
	}

	var (
		vm      *payloads.VM
		blocker string
		noted   bool
	)
	for {
		vm, err = xo.VM().GetByID(ctx, id)
		if err != nil {
			if ctx.Err() != nil {
				return endedWait(ctx, name, waitTimeout, blocker)
			}
			return notFound(idArg, err, cfg.Insecure)
		}
		blocker = gate.check(ctx, vm)
		if blocker == "" {
			return renderVMWait(cmd, name, id, vm, gate, query)
		}
		if !noted {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Waiting for VM %q to be ready (%s)...\n", name, blocker); err != nil {
				return err
			}
			noted = true
		}
		select {
		case <-ctx.Done():
			return endedWait(ctx, name, waitTimeout, blocker)
		case <-time.After(taskwait.PollInterval):
		}
	}
}

// endedWait turns an ended wait into a concise error. A deadline is reported
// as a timeout (with what was still missing); a cancellation (Ctrl+C) as an
// interruption.
func endedWait(ctx context.Context, name string, waitTimeout time.Duration, blocker string) error {
	if ctx.Err() == context.DeadlineExceeded {
		if blocker != "" {
			return fmt.Errorf("VM %q was not ready within %s (%s)", name, waitTimeout, blocker)
		}
		return fmt.Errorf("VM %q was not ready within %s", name, waitTimeout)
	}
	return fmt.Errorf("waiting for VM %q was interrupted", name)
}

// renderVMWait renders the outcome of a successful wait. The machine document
// is a small readiness document — not the full VM object, which 'xo vm get
// --output json' already provides: just what the next step needs (the name,
// the state, the IP to connect to).
func renderVMWait(cmd *cobra.Command, name string, id uuid.UUID, vm *payloads.VM, gate vmWaitGate, query string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()

	doc := map[string]any{
		"vm":    name,
		"id":    id.String(),
		"state": vmState(vm),
		"ip":    vm.MainIpAddress,
	}
	if gate.ssh {
		doc["port"] = gate.port
		doc["ssh"] = true
	}

	if query != "" {
		queryResult, err := output.Query(query, doc)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, doc, queryResult)
	}

	switch format {
	case output.FormatJSON, output.FormatYAML:
		normalized, err := output.Normalize(doc)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, normalized, nil)
	default:
		out := fmt.Sprintf("VM %q is ready:\n  id:     %s\n  state:  %s\n  ip:     %s\n",
			name, id.String(), vmState(vm), vm.MainIpAddress)
		if gate.ssh {
			out += fmt.Sprintf("  ssh:    reachable on %s:%d\n\nConnect with: ssh <user>@%s\n",
				vm.MainIpAddress, gate.port, vm.MainIpAddress)
		}
		_, err := fmt.Fprint(w, out)
		return err
	}
}
