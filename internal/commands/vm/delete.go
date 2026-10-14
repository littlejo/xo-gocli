package vm

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

func newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a virtual machine",
		Long: `Delete a virtual machine.

This removes the VM from Xen Orchestra and frees the disks and memory
associated with it. The operation is irreversible: the VM's data is lost.
A running VM must be stopped or suspended first (the API refuses to delete
it otherwise).

This is a destructive operation and asks for confirmation unless --yes is
given. The VM is referenced by its UUID, as returned by 'xo vm list'.`,
		Example: `  xo vm delete 550e8400-e29b-41d4-a716-446655440001
  xo vm delete <id> --yes`,
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

// runDelete implements the delete flow: load the client, resolve the target VM
// (as an existence check), confirm, delete, and print a concise confirmation.
// It mirrors runAction but without a task id: the SDK's Delete is synchronous
// and returns only an error.
func runDelete(cmd *cobra.Command, idStr string, yes bool) error {
	xo, cfg, err := newClient(cmd)
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
		return cli.NotFound("VM", "resolve", idStr, err, cfg.Insecure)
	}

	ok, err := confirm(cmd, fmt.Sprintf("Are you sure you want to delete VM %q?", name), yes)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}

	if err := xo.VM().Delete(ctx, id); err != nil {
		return cli.InsecureHint(fmt.Sprintf("cannot delete VM %q: %v", name, err), cfg.Insecure)
	}

	return renderDeleted(cmd, name)
}

// renderDeleted prints the outcome of a VM delete: a friendly line for the
// human formats and a small machine document for json/yaml.
func renderDeleted(cmd *cobra.Command, name string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"action": "delete", "vm": name})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Deleted VM %q\n", name)
		return err
	}
}
