// Package template implements the 'xo template' command group.
//
// In Xen Orchestra, VM templates are first-class objects exposed by the
// dedicated REST resource "vm-templates"; they are not part of the "vms"
// collection (so 'xo vm list' does not show them). The SDK v2 does not (yet)
// wrap that resource with a typed service, so this command reaches it through
// the SDK's own REST client (cli.NewHTTPClient) — the same single API
// boundary, not a second HTTP client. The missing operation should be
// contributed to the SDK.
package template

import (
	"github.com/spf13/cobra"
)

// NewCommand builds the 'xo template' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage VM templates",
		Long: `List and inspect the VM templates available in each pool. A template id
printed here is what 'xo vm create --template' takes.`,
		Example: `  xo template list
  xo template list --query '[].name_label'
  xo template get <id>`,
	}
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	return cmd
}
