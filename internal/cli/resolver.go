package cli

import (
	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

// NewResolver builds the shared name resolver for a command that holds no
// typed SDK client yet. A command that already has one passes it to
// resolve.New instead, so it builds no second client.
func NewResolver(cmd *cobra.Command, cfg *config.ClientConfig) (*resolve.Client, error) {
	lib, err := NewClient(cmd, cfg)
	if err != nil {
		return nil, err
	}
	return resolve.New(lib), nil
}
