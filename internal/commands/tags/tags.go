// Package tags builds the '<resource> tag' command group (add / remove) shared
// by every resource whose SDK v2 typed service exposes AddTag / RemoveTag
// (PUT/DELETE on /rest/v0/<collection>/{id}/tags/{tag}). Each resource package
// supplies only what differs: its names and the SDK calls.
package tags

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// Spec describes the tagged resource.
type Spec struct {
	// Group is the command group, as typed by the user: "host" in 'xo host tag'.
	Group string
	// Kind names the resource in messages and errors: "host", "SR", "VM".
	Kind string
	// Noun names the resource in the help text: "a host", "a storage repository".
	Noun string
	// ExampleID is the UUID shown in the help examples.
	ExampleID string

	// NameOf returns the name_label of the resource; it doubles as the
	// existence check, run before the tag operation.
	NameOf func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error)
	// Add and Remove are the SDK v2 tag operations.
	Add    func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error
	Remove func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error
}

// NewCommand builds the 'xo <group> tag' group with its add and remove
// subcommands.
func NewCommand(spec Spec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage the tags of " + spec.Noun,
		Long: fmt.Sprintf(`Add or remove the tags of %s. Tags are free-form labels; filter on
them with --query, for example:

  xo %s list --query "[?contains(tags, 'production')].name_label"`, spec.Noun, spec.Group),
		Example: fmt.Sprintf(`  xo %[1]s tag add <id> production
  xo %[1]s tag remove <id> production`, spec.Group),
	}
	cmd.AddCommand(newSubcommand(spec, operation{
		verb: "add", title: "Add", past: "added", preposition: "to", perform: spec.Add,
	}))
	cmd.AddCommand(newSubcommand(spec, operation{
		verb: "remove", title: "Remove", past: "removed", preposition: "from", perform: spec.Remove,
	}))
	return cmd
}

// operation is one tag subcommand: the verbs used in help and messages, and
// the SDK call to run.
type operation struct {
	verb        string // base form, used in error messages ("cannot add tag ...")
	title       string // capitalized form, used in the help ("Add a tag ...")
	past        string // past form, used in the confirmation ("Tag ... added on ...")
	preposition string // used in the help ("Add a tag to ...")
	perform     func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error
}

func newSubcommand(spec Spec, op operation) *cobra.Command {
	return &cobra.Command{
		Use:   op.verb + " <id> <tag>",
		Short: fmt.Sprintf("%s a tag %s %s", op.title, op.preposition, spec.Noun),
		Long: fmt.Sprintf(`%s a tag %s %s.

The %s is referenced by its UUID, as returned by 'xo %s list'.`, op.title, op.preposition, spec.Noun, spec.Kind, spec.Group),
		Example: fmt.Sprintf(`  xo %[1]s tag %[2]s %[3]s production
  xo %[1]s tag %[2]s <id> web`, spec.Group, op.verb, spec.ExampleID),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, spec, op, args[0], args[1])
		},
	}
}

// run resolves the target resource (as an existence check and to display its
// name), applies the tag operation, and prints a concise confirmation.
func run(cmd *cobra.Command, spec Spec, op operation, idStr, tag string) error {
	xo, cfg, err := cli.LoadClient(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	id, err := cli.ParseID(spec.Kind, idStr)
	if err != nil {
		return err
	}
	name, err := spec.NameOf(ctx, xo, id)
	if err != nil {
		return cli.NotFound(spec.Kind, "resolve", idStr, err, cfg.Insecure)
	}
	if name == "" {
		name = id.String()
	}

	if err := op.perform(ctx, xo, id, tag); err != nil {
		return cli.APIError(err, cfg.Insecure, "cannot %s tag %q on %s %q", op.verb, tag, spec.Kind, name)
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Tag %q %s on %s %q\n", tag, op.past, spec.Kind, name)
	return err
}
