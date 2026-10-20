package vm

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/taskwait"
)

// flagYes skips the interactive confirmation for destructive operations. It is
// required whenever stdin is not a terminal so automation never blocks; the
// $XOA_YES environment variable is an equivalent for scripts (cli.SkipConfirm).
const flagYes = "yes"

// parseID converts a positional VM identifier into a UUID.
func parseID(id string) (uuid.UUID, error) {
	return cli.ParseID("VM", id)
}

// nameOf resolves the name_label of the VM with the given id. It also acts as
// an existence check so actions fail with a clear "not found" error before the
// operation is performed.
func nameOf(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
	vm, err := xo.VM().GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if vm.NameLabel == "" {
		return id.String(), nil
	}
	return vm.NameLabel, nil
}

// notFound delegates to cli.NotFound, which reports a 404 concisely and keeps
// the raw API error as debug detail.
func notFound(id string, err error, insecure bool) error {
	return cli.NotFound("VM", "get", id, err, insecure)
}

// actionSpec describes an async VM action so each command only supplies what
// differs: the verb shown to the user, whether it is destructive, and the SDK
// call to run.
type actionSpec struct {
	verb        string
	id          string
	destructive bool
	yes         bool
	perform     func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error)
}

// runAction implements the common flow shared by start/stop/reboot/snapshot:
// load the client, resolve and confirm the target VM, run the action, and print
// the resulting task ID.
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
		return cli.NotFound("VM", "resolve", spec.id, err, cfg.Insecure)
	}

	if spec.destructive {
		ok, err := cli.Confirm(cmd, fmt.Sprintf("Are you sure you want to %s VM %q?", spec.verb, name), spec.yes)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("aborted")
		}
	}

	taskID, err := spec.perform(ctx, xo, id)
	if err != nil {
		return cli.InsecureHint(fmt.Sprintf("cannot %s VM %q: %v", spec.verb, name, err), cfg.Insecure)
	}

	if cli.WaitEnabled(cmd) && taskID != "" {
		return taskwait.ForCommand(ctx, cmd, cfg, taskID)
	}

	return taskwait.RenderStarted(cmd, "vm", spec.verb, name, strconv.Quote(name), taskID)
}
