package vm

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// newTagCommand builds the 'xo vm tag' group, which exposes the add and
// remove subcommands. Tags are managed through the SDK v2 typed service
// (VM().AddTag / VM().RemoveTag), which maps to PUT/DELETE on
// /rest/v0/vms/{id}/tags/{tag}.
func newTagCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage the tags of a virtual machine",
		Long: `Add or remove the tags of a virtual machine. Tags are free-form labels; filter on
them with --query, for example:

  xo vm list --query "[?contains(tags, 'production')].name_label"`,
		Example: `  xo vm tag add <id> production
  xo vm tag remove <id> production`,
	}
	cmd.AddCommand(newTagAddCommand())
	cmd.AddCommand(newTagRemoveCommand())
	return cmd
}

// tagSpec captures what a tag subcommand needs: the verb shown to the user and
// the SDK call to run.
type tagSpec struct {
	verb    string // base form, used in error messages ("cannot add tag ...")
	past    string // past form, used in the confirmation ("Tag ... added on ...")
	tag     string
	perform func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error
}

// runTag resolves the target VM (as an existence check), applies the tag
// operation, and prints a concise confirmation.
func runTag(cmd *cobra.Command, idStr string, spec tagSpec) error {
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

	if err := spec.perform(ctx, xo, id, spec.tag); err != nil {
		return cli.InsecureHint(fmt.Sprintf("cannot %s tag %q on VM %q: %v", spec.verb, spec.tag, name, err), cfg.Insecure)
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Tag %q %s on VM %q\n", spec.tag, spec.past, name)
	return err
}

func newTagAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <id> <tag>",
		Short: "Add a tag to a virtual machine",
		Long: `Add a tag to a virtual machine.

The VM is referenced by its UUID, as returned by 'xo vm list'.`,
		Example: `  xo vm tag add 550e8400-e29b-41d4-a716-446655440001 production
  xo vm tag add <id> web`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTag(cmd, args[0], tagSpec{
				verb: "add",
				past: "added",
				tag:  args[1],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
					return xo.VM().AddTag(ctx, id, tag)
				},
			})
		},
	}
	return cmd
}

func newTagRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <id> <tag>",
		Short: "Remove a tag from a virtual machine",
		Long: `Remove a tag from a virtual machine.

The VM is referenced by its UUID, as returned by 'xo vm list'.`,
		Example: `  xo vm tag remove 550e8400-e29b-41d4-a716-446655440001 production
  xo vm tag remove <id> web`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTag(cmd, args[0], tagSpec{
				verb: "remove",
				past: "removed",
				tag:  args[1],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
					return xo.VM().RemoveTag(ctx, id, tag)
				},
			})
		},
	}
	return cmd
}
