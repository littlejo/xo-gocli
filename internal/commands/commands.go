// Package commands assembles the xo command tree.
package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/commands/configure"
	"github.com/littlejo/xo-gocli/internal/commands/host"
	"github.com/littlejo/xo-gocli/internal/commands/network"
	"github.com/littlejo/xo-gocli/internal/commands/pbd"
	"github.com/littlejo/xo-gocli/internal/commands/pool"
	"github.com/littlejo/xo-gocli/internal/commands/rest"
	"github.com/littlejo/xo-gocli/internal/commands/sr"
	"github.com/littlejo/xo-gocli/internal/commands/task"
	"github.com/littlejo/xo-gocli/internal/commands/template"
	"github.com/littlejo/xo-gocli/internal/commands/token"
	"github.com/littlejo/xo-gocli/internal/commands/vbd"
	"github.com/littlejo/xo-gocli/internal/commands/vdi"
	"github.com/littlejo/xo-gocli/internal/commands/vm"
)

// NewRoot builds the root command with its global flags and subcommands.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "xo",
		Short: "Xen Orchestra command line client",
		Long: `xo is a command line client for Xen Orchestra.

It talks to the Xen Orchestra REST API through the official Go SDK (v2).
Configure a profile first with 'xo configure'.`,
		Version:       cli.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringP(cli.FlagProfile, "p", "", "name of the configuration profile to use (or $XOA_PROFILE)")
	root.PersistentFlags().StringP(cli.FlagOutput, "o", "table", "output format: table, json, yaml, text (or $XOA_DEFAULT_OUTPUT, or the profile's \"output\" value)")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "output as JSON (shorthand for --output json)")
	root.PersistentFlags().BoolP(cli.FlagDebug, "d", false, "show SDK/API error details on failure (or $XOA_DEBUG)")
	root.PersistentFlags().Duration(cli.FlagTimeout, 0, "HTTP client timeout, e.g. 60s or 2m (default 30s, or $XOA_TIMEOUT)")
	// Defined here so cobra does not add its default -v shorthand: -v stays
	// free (e.g. for a future --verbose). Cobra still handles --version.
	root.Flags().Bool("version", false, "print the xo CLI version")

	root.AddCommand(configure.NewCommand())
	root.AddCommand(newVersionCommand())
	root.AddCommand(host.NewCommand())
	root.AddCommand(network.NewCommand())
	root.AddCommand(pbd.NewCommand())
	root.AddCommand(pool.NewCommand())
	root.AddCommand(rest.NewCommand())
	root.AddCommand(sr.NewCommand())
	root.AddCommand(task.NewCommand())
	root.AddCommand(template.NewCommand())
	root.AddCommand(token.NewCommand())
	root.AddCommand(vbd.NewCommand())
	root.AddCommand(vdi.NewCommand())
	root.AddCommand(vm.NewCommand())

	for _, c := range root.Commands() {
		rejectUnknownSubcommands(c)
	}

	return root
}

// rejectUnknownSubcommands makes every command group below the root (xo vm,
// xo vm tag, …) fail on an unknown subcommand. Cobra only checks unknown
// subcommands on the root: a non-runnable group prints its help and exits 0
// on 'xo vm strat <id>', so a typo in a script passes as a success. Giving
// each group a RunE turns that into a usage error (exit 1); bare 'xo vm'
// still prints the help and exits 0.
func rejectUnknownSubcommands(cmd *cobra.Command) {
	for _, c := range cmd.Commands() {
		rejectUnknownSubcommands(c)
	}
	if cmd.Runnable() || !cmd.HasSubCommands() {
		return
	}
	// Cobra only defaults this (to 2) inside its own root-level check.
	cmd.SuggestionsMinimumDistance = 2
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		msg := fmt.Sprintf("unknown command %q for %q", args[0], cmd.CommandPath())
		if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
			msg += fmt.Sprintf(" (did you mean %s?)", strings.Join(suggestions, ", "))
		}
		return fmt.Errorf("%s, run '%s --help' for the available commands", msg, cmd.CommandPath())
	}
}
