package sr

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/taskwait"
)

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
		return taskwait.ForCommand(ctx, cmd, cfg, taskID)
	}

	return taskwait.RenderStarted(cmd, "sr", spec.verb, name, strconv.Quote(name), taskID)
}
