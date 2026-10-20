package sr

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// newTagCommand builds the 'xo sr tag' group (add / remove). Tags are managed
// through the SDK v2 typed service (SR().AddTag / SR().RemoveTag), which maps
// to PUT/DELETE on /rest/v0/srs/{id}/tags/{tag}.
func newTagCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage the tags of a storage repository",
		Long: `Add or remove the tags of a storage repository. Tags are free-form labels; filter on
them with --query, for example:

  xo sr list --query "[?contains(tags, 'production')].name_label"`,
		Example: `  xo sr tag add <id> production
  xo sr tag remove <id> production`,
	}
	cmd.AddCommand(newTagAddCommand())
	cmd.AddCommand(newTagRemoveCommand())
	return cmd
}

// tagSpec captures what a tag subcommand needs: the verbs used in messages,
// the existence/name lookup and the SDK call to run.
type tagSpec struct {
	verb    string // base form, used in error messages ("cannot add tag ...")
	past    string // past form, used in the confirmation ("Tag ... added on ...")
	tag     string
	nameOf  func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error)
	perform func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error
}

// runTag resolves the target SR (as an existence check and to display its
// name), applies the tag operation, and prints a concise confirmation.
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
	name, err := spec.nameOf(ctx, xo, id)
	if err != nil {
		return cli.NotFound("SR", "resolve", idStr, err, cfg.Insecure)
	}

	if err := spec.perform(ctx, xo, id, spec.tag); err != nil {
		return cli.InsecureHint(fmt.Sprintf("cannot %s tag %q on SR %q: %v", spec.verb, spec.tag, name, err), cfg.Insecure)
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Tag %q %s on SR %q\n", spec.tag, spec.past, name)
	return err
}

func newTagAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <id> <tag>",
		Short: "Add a tag to a storage repository",
		Long: `Add a tag to a storage repository.

The SR is referenced by its UUID, as returned by 'xo sr list'.`,
		Example: `  xo sr tag add 11111111-1111-4111-8111-111111111111 production
  xo sr tag add <id> web`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTag(cmd, args[0], tagSpec{
				verb: "add",
				past: "added",
				tag:  args[1],
				nameOf: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
					s, err := xo.SR().Get(ctx, id)
					if err != nil {
						return "", err
					}
					return s.NameLabel, nil
				},
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
					return xo.SR().AddTag(ctx, id, tag)
				},
			})
		},
	}
	return cmd
}

func newTagRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <id> <tag>",
		Short: "Remove a tag from a storage repository",
		Long: `Remove a tag from a storage repository.

The SR is referenced by its UUID, as returned by 'xo sr list'.`,
		Example: `  xo sr tag remove 11111111-1111-4111-8111-111111111111 production
  xo sr tag remove <id> web`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTag(cmd, args[0], tagSpec{
				verb: "remove",
				past: "removed",
				tag:  args[1],
				nameOf: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
					s, err := xo.SR().Get(ctx, id)
					if err != nil {
						return "", err
					}
					return s.NameLabel, nil
				},
				perform: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
					return xo.SR().RemoveTag(ctx, id, tag)
				},
			})
		},
	}
	return cmd
}
