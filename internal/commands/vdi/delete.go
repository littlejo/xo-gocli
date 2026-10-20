package vdi

import (
	"context"
	"errors"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

// flagYes skips the interactive confirmation for destructive operations. It is
// required whenever stdin is not a terminal so automation never blocks; the
// $XOA_YES environment variable is an equivalent for scripts (cli.SkipConfirm).
const flagYes = "yes"

func newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a virtual disk (VDI)",
		Long: `Delete a virtual disk (VDI).

This removes the VDI and the data it contains from the storage repository.
The operation is irreversible. The VDI must not be attached to any VM:
detach it first with 'xo vbd delete <vbd-id>' (the API refuses to delete an
attached VDI otherwise).

This is a destructive operation and asks for confirmation unless --yes is
given. The VDI is referenced by its UUID, as returned by 'xo vdi list'.`,
		Example: `  xo vdi delete 11111111-1111-4111-8111-111111111111
  xo vdi delete <id> --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, args[0], cli.SkipConfirm(cmd))
		},
	}
	// --yes (or $XOA_YES) skips the confirmation; it is read via
	// cli.SkipConfirm, which sees both the flag and the environment variable.
	cmd.Flags().Bool(flagYes, false, "do not ask for confirmation (or set XOA_YES=1)")
	return cmd
}

// nameOf resolves the name_label of the VDI with the given id. It also acts as
// an existence check so the delete fails with a clear "not found" error before
// the operation is performed.
func nameOf(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
	vdi, err := xo.VDI().Get(ctx, id)
	if err != nil {
		return "", err
	}
	if vdi.NameLabel == "" {
		return id.String(), nil
	}
	return vdi.NameLabel, nil
}

// runDelete implements the delete flow: load the client, resolve the target
// VDI (as an existence check), confirm, delete, and print a concise
// confirmation. The SDK's Delete is synchronous and returns only an error.
func runDelete(cmd *cobra.Command, idStr string, yes bool) error {
	xo, cfg, err := cli.LoadClient(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	id, err := parseID(idStr)
	if err != nil {
		return err
	}
	name, err := nameOf(ctx, xo, id)
	if err != nil {
		return cli.NotFound("VDI", "resolve", idStr, err, cfg.Insecure)
	}

	ok, err := cli.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete VDI %q?", name), yes)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}

	if err := xo.VDI().Delete(ctx, id); err != nil {
		return cli.APIError(err, cfg.Insecure, "cannot delete VDI %q", name)
	}

	return renderDeleted(cmd, name)
}

// renderDeleted prints the outcome of a VDI delete: a friendly line for the
// human formats and a small machine document for json/yaml.
func renderDeleted(cmd *cobra.Command, name string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"action": "delete", "vdi": name})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Deleted VDI %q\n", name)
		return err
	}
}
