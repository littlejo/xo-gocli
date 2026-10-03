package cli

import (
	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

// NewResolver builds the shared name resolver used by the `get` detail views
// to display relationships by name instead of UUID. It needs both the typed
// SDK v2 service facade and the SDK REST client (for the endpoints the typed
// services do not wrap yet, such as vm-templates); authentication, base URL
// and TLS handling all come from the SDK, so this is the same single API
// boundary as the rest of the CLI.
func NewResolver(cmd *cobra.Command, cfg *config.ClientConfig) (*resolve.Client, error) {
	httpClient, err := NewHTTPClient(cmd, cfg)
	if err != nil {
		return nil, err
	}
	lib, err := NewClient(cmd, cfg)
	if err != nil {
		return nil, err
	}
	return resolve.New(lib, httpClient), nil
}
