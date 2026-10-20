package vm

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/commands/tags"
)

// newTagCommand builds the 'xo vm tag' group (add / remove), through the SDK
// v2 typed service (VM().AddTag / VM().RemoveTag).
func newTagCommand() *cobra.Command {
	return tags.NewCommand(tags.Spec{
		Group:     "vm",
		Kind:      "VM",
		Noun:      "a virtual machine",
		ExampleID: "550e8400-e29b-41d4-a716-446655440001",
		NameOf:    nameOf,
		Add: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.VM().AddTag(ctx, id, tag)
		},
		Remove: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.VM().RemoveTag(ctx, id, tag)
		},
	})
}
