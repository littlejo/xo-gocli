package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
)

// Candidate is one shell-completion value: an object id, and the name the
// shells that show descriptions (zsh, fish, PowerShell) print beside it.
type Candidate struct {
	ID   string
	Name string
}

// CompleteIDs makes every command under group (at any depth) whose first
// argument is an object id — "<id>" or "<id|name>" in its Use, such as
// 'xo vm start <id>' or 'xo vm tag add <id> <tag>' — complete that argument
// with the ids list returns. A command that also takes a name ("<id|name>")
// completes the names too, except a name several objects share, which it
// would reject. Commands that already complete their arguments are left
// alone. Call it once the group holds all its subcommands.
//
// Completion costs one list request, with the profile and --timeout of the
// command line being completed. A failure (no profile, unreachable server)
// completes nothing rather than printing an error into the shell.
func CompleteIDs(group *cobra.Command, list func(ctx context.Context, xo library.Library) ([]Candidate, error)) {
	complete := func(withNames bool) cobra.CompletionFunc {
		return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
			return completeIDs(cmd, args, toComplete, withNames, list)
		}
	}

	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, sub := range cmd.Commands() {
			fields := strings.Fields(sub.Use)
			if sub.ValidArgsFunction == nil && len(fields) > 1 && (fields[1] == "<id>" || fields[1] == "<id|name>") {
				sub.ValidArgsFunction = complete(fields[1] == "<id|name>")
			}
			walk(sub)
		}
	}
	walk(group)
}

// completeIDs is the completion function CompleteIDs installs: ids that
// start with toComplete (the name as description) and, withNames, the names
// only one object has (the id as description).
func completeIDs(cmd *cobra.Command, args []string, toComplete string, withNames bool,
	list func(ctx context.Context, xo library.Library) ([]Candidate, error),
) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	xo, _, err := LoadClient(cmd)
	if err != nil {
		cobra.CompDebugln("cannot complete ids: "+err.Error(), false)
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	candidates, err := list(cmd.Context(), xo)
	if err != nil {
		cobra.CompDebugln("cannot complete ids: "+err.Error(), false)
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var out []cobra.Completion
	uses := map[string]int{}
	for _, c := range candidates {
		uses[c.Name]++
		if strings.HasPrefix(c.ID, toComplete) {
			out = append(out, cobra.CompletionWithDesc(c.ID, c.Name))
		}
	}
	if withNames {
		for _, c := range candidates {
			if c.Name != "" && uses[c.Name] == 1 && strings.HasPrefix(c.Name, toComplete) {
				out = append(out, cobra.CompletionWithDesc(c.Name, c.ID))
			}
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
