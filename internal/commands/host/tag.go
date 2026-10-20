package host

import (
	"context"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/commands/tags"
)

// newTagCommand builds the 'xo host tag' group (add / remove), through the SDK
// v2 typed service (Host().AddTag / Host().RemoveTag).
func newTagCommand() *cobra.Command {
	return tags.NewCommand(tags.Spec{
		Group:     "host",
		Kind:      "host",
		Noun:      "a host",
		ExampleID: "aaaaaaaa-bbbb-cccc-dddd-000000000001",
		NameOf: func(ctx context.Context, xo library.Library, id uuid.UUID) (string, error) {
			h, err := xo.Host().Get(ctx, id)
			if err != nil {
				return "", err
			}
			return h.NameLabel, nil
		},
		Add: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.Host().AddTag(ctx, id, tag)
		},
		Remove: func(ctx context.Context, xo library.Library, id uuid.UUID, tag string) error {
			return xo.Host().RemoveTag(ctx, id, tag)
		},
	})
}
