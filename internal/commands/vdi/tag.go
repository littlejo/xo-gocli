package vdi

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/commands/tags"
)

// newTagCommand builds the 'xo vdi tag' group (add / remove), through the SDK
// v2 typed service (VDI().AddTag / VDI().RemoveTag).
func newTagCommand() *cobra.Command {
	return tags.NewCommand(tags.Spec{
		Group:     "vdi",
		Kind:      "VDI",
		Noun:      "a virtual disk (VDI)",
		ExampleID: "11111111-1111-4111-8111-111111111111",
		NameOf: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
			v, err := xo.VDI().Get(ctx, id)
			if err != nil {
				return "", err
			}
			return v.NameLabel, nil
		},
		Add: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.VDI().AddTag(ctx, id, tag)
		},
		Remove: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.VDI().RemoveTag(ctx, id, tag)
		},
	})
}
