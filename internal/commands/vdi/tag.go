package vdi

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// newTagCommand builds the 'xo vdi tag' group (add / remove). Tags are managed
// through the SDK v2 typed service (VDI().AddTag / VDI().RemoveTag), which maps
// to PUT/DELETE on /rest/v0/vdis/{id}/tags/{tag}.
func newTagCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage the tags of a virtual disk (VDI)",
		Long: `Add or remove the tags of a virtual disk (VDI). Tags are free-form labels; filter on
them with --query, for example:

  xo vdi list --query "[?contains(tags, 'production')].name_label"`,
		Example: `  xo vdi tag add <id> production
  xo vdi tag remove <id> production`,
	}
	cmd.AddCommand(newTagAddCommand())
	cmd.AddCommand(newTagRemoveCommand())
	return cmd
}

// tagSpec captures what a tag subcommand needs: the verbs used in messages and
// the SDK call to run.
type tagSpec struct {
	verb    string // base form, used in error messages ("cannot add tag ...")
	past    string // past form, used in the confirmation ("Tag ... added on ...")
	tag     string
	perform func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error
}

// runTag applies the tag operation on the VDI (which is resolved as an
// existence check so failures report a clear "not found" error) and prints a
// concise confirmation.
func runTag(cmd *cobra.Command, idStr string, spec tagSpec) error {
	xo, cfg, err := cli.LoadClient(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	id, err := parseID(idStr)
	if err != nil {
		return err
	}
	vdi, err := xo.VDI().Get(ctx, id)
	if err != nil {
		return notFound(idStr, err, cfg.Insecure)
	}
	name := vdi.NameLabel
	if name == "" {
		name = id.String()
	}

	if err := spec.perform(ctx, xo, id, spec.tag); err != nil {
		return cli.InsecureHint(fmt.Sprintf("cannot %s tag %q on VDI %q: %v", spec.verb, spec.tag, name, err), cfg.Insecure)
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Tag %q %s on VDI %q\n", spec.tag, spec.past, name)
	return err
}

func newTagAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <id> <tag>",
		Short: "Add a tag to a virtual disk (VDI)",
		Long: `Add a tag to a virtual disk (VDI).

The VDI is referenced by its UUID, as returned by 'xo vdi list'.`,
		Example: `  xo vdi tag add 11111111-1111-4111-8111-111111111111 production
  xo vdi tag add <id> web`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTag(cmd, args[0], tagSpec{
				verb: "add",
				past: "added",
				tag:  args[1],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
					return xo.VDI().AddTag(ctx, id, tag)
				},
			})
		},
	}
	return cmd
}

func newTagRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <id> <tag>",
		Short: "Remove a tag from a virtual disk (VDI)",
		Long: `Remove a tag from a virtual disk (VDI).

The VDI is referenced by its UUID, as returned by 'xo vdi list'.`,
		Example: `  xo vdi tag remove 11111111-1111-4111-8111-111111111111 production
  xo vdi tag remove <id> web`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTag(cmd, args[0], tagSpec{
				verb: "remove",
				past: "removed",
				tag:  args[1],
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
					return xo.VDI().RemoveTag(ctx, id, tag)
				},
			})
		},
	}
	return cmd
}
