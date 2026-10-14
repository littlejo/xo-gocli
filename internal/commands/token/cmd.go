// Package token implements the 'xo token' command group.
package token

import (
	"github.com/spf13/cobra"
)

// NewCommand builds the 'xo token' command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage authentication tokens",
		Long: `List, inspect and create the authentication tokens of the current user.
A token is the recommended way to authenticate: store it with
'xo configure --token'.

The token id is the secret: it is printed in full once, by 'create', and
masked everywhere else.`,
		Example: `  xo token create --description "ci" --expires-in "30 days"
  xo token list`,
	}
	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newListCommand())
	return cmd
}
