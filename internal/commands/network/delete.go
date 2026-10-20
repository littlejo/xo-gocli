package network

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
		Short: "Delete a network",
		Long: `Delete a network.

This removes the network from the pool. The operation is irreversible and
refused while the network still has attached virtual interfaces (VIFs): the
connected VMs must be dealt with first.

This is a destructive operation and asks for confirmation unless --yes is
given. The network is referenced by its UUID, as returned by 'xo network list'.`,
		Example: `  xo network delete 11111111-1111-4111-8111-111111111111
  xo network delete <id> --yes`,
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

// nameOf resolves the name_label of the network with the given id. It also
// acts as an existence check so the delete fails with a clear "not found"
// error before the operation is performed.
func nameOf(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
	network, err := xo.Network().Get(ctx, id)
	if err != nil {
		return "", err
	}
	if network.NameLabel == "" {
		return id.String(), nil
	}
	return network.NameLabel, nil
}

// runDelete implements the delete flow: load the client, resolve the target
// network (as an existence check), confirm, delete, and print a concise
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
		return cli.NotFound("network", "resolve", idStr, err, cfg.Insecure)
	}

	ok, err := cli.Confirm(cmd, fmt.Sprintf("Are you sure you want to delete network %q?", name), yes)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}

	if err := xo.Network().Delete(ctx, id); err != nil {
		return cli.APIError(err, cfg.Insecure, "cannot delete network %q", name)
	}

	return renderDeleted(cmd, name)
}

// renderDeleted prints the outcome of a network delete: a friendly line for
// the human formats and a small machine document for json/yaml.
func renderDeleted(cmd *cobra.Command, name string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"action": "delete", "network": name})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Deleted network %q\n", name)
		return err
	}
}
