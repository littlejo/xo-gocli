package sr

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/commands/tags"
)

// newTagCommand builds the 'xo sr tag' group (add / remove), through the SDK
// v2 typed service (SR().AddTag / SR().RemoveTag).
func newTagCommand() *cobra.Command {
	return tags.NewCommand(tags.Spec{
		Group:     "sr",
		Kind:      "SR",
		Noun:      "a storage repository",
		ExampleID: "11111111-1111-4111-8111-111111111111",
		NameOf: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
			s, err := xo.SR().Get(ctx, id)
			if err != nil {
				return "", err
			}
			return s.NameLabel, nil
		},
		Add: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.SR().AddTag(ctx, id, tag)
		},
		Remove: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.SR().RemoveTag(ctx, id, tag)
		},
	})
}
