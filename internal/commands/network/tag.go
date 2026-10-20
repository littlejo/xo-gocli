package network

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/commands/tags"
)

// newTagCommand builds the 'xo network tag' group (add / remove), through the SDK
// v2 typed service (Network().AddTag / Network().RemoveTag).
func newTagCommand() *cobra.Command {
	return tags.NewCommand(tags.Spec{
		Group:     "network",
		Kind:      "network",
		Noun:      "a network",
		ExampleID: "11111111-1111-4111-8111-111111111111",
		NameOf: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
			n, err := xo.Network().Get(ctx, id)
			if err != nil {
				return "", err
			}
			return n.NameLabel, nil
		},
		Add: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.Network().AddTag(ctx, id, tag)
		},
		Remove: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.Network().RemoveTag(ctx, id, tag)
		},
	})
}
