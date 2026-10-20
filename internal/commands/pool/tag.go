package pool

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/commands/tags"
)

// newTagCommand builds the 'xo pool tag' group (add / remove), through the SDK
// v2 typed service (Pool().AddTag / Pool().RemoveTag).
func newTagCommand() *cobra.Command {
	return tags.NewCommand(tags.Spec{
		Group:     "pool",
		Kind:      "pool",
		Noun:      "a pool",
		ExampleID: "aaaaaaaa-bbbb-cccc-dddd-000000000001",
		NameOf: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
			p, err := xo.Pool().Get(ctx, id)
			if err != nil {
				return "", err
			}
			return p.NameLabel, nil
		},
		Add: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.Pool().AddTag(ctx, id, tag)
		},
		Remove: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.Pool().RemoveTag(ctx, id, tag)
		},
	})
}
