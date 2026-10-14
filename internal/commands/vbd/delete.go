package vbd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

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
		Short: "Detach a VDI from a VM (delete the VBD)",
		Long: `Delete a virtual block device (VBD), detaching the VDI from the VM.

Only the attachment is removed; the VDI (and the data it contains) is kept on
the storage repository. To also delete the disk, run 'xo vdi delete <vdi-id>'
afterwards (the VDI must have no remaining VBD).

This is a destructive operation and asks for confirmation unless --yes is
given. The VBD is referenced by its UUID, as returned by 'xo vbd list'.`,
		Example: `  xo vbd delete 33333333-3333-4333-8333-333333333333
  xo vbd delete <id> --yes`,
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

// describe returns a short human-readable label for a VBD (it has no name):
// the VM and VDI ids it links, or the VBD id when the VDI is unset.
func describe(vbd *payloads.VBD) string {
	if vdiText(vbd.VDI) == "-" {
		return vbd.ID.String()
	}
	return fmt.Sprintf("VBD %s (VM %s, VDI %s)", vbd.ID.String(), vbd.VM.String(), vdiText(vbd.VDI))
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

// runDelete implements the delete flow: load the client, resolve the target
// VBD (as an existence check), confirm, delete, and print a concise
// confirmation. The SDK's Delete is synchronous and returns only an error.
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
	vbd, err := xo.VBD().Get(ctx, id)
	if err != nil {
		return cli.NotFound("VBD", "resolve", idStr, err, cfg.Insecure)
	}
	label := describe(vbd)

	ok, err := confirm(cmd, fmt.Sprintf("Are you sure you want to detach %s?", label), yes)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}

	if err := xo.VBD().Delete(ctx, id); err != nil {
		return cli.InsecureHint(fmt.Sprintf("cannot delete VBD %s: %v", label, err), cfg.Insecure)
	}

	return renderDeleted(cmd, label)
}

// renderDeleted prints the outcome of a VBD delete: a friendly line for the
// human formats and a small machine document for json/yaml.
func renderDeleted(cmd *cobra.Command, label string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"action": "delete", "vbd": label})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Detached %s (the VDI is kept; use 'xo vdi delete' to remove it)\n", label)
		return err
	}
}
