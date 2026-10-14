package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// newVersionCommand implements 'xo version'. It prints the CLI version,
// offline, like 'aws version' or 'kubectl version --client'. The Xen
// Orchestra server version is not exposed by the REST API (v0) — only as a
// JSON-RPC v1 method, which this CLI never uses — so there is nothing to
// connect to in order to display it.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the xo CLI version",
		Long: `Print the version of the xo CLI.

This works offline, like 'aws version' or 'kubectl version --client'.
It prints the same line as '--version', so both can be used
interchangeably in scripts.

The Xen Orchestra server version is not shown: the REST API (v0) does not
expose it, and the CLI never uses the legacy JSON-RPC API.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Same line as cobra's '--version' output.
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "xo version "+cli.Version)
			return err
		},
	}
}
