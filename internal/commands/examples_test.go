package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestHelpTextConventions pins the help layout every command follows:
// examples live in the cobra Example field (so --help prints them after
// Usage, and generated references get an Examples section), never inside
// Long; and every command but the auto-generated ones has a Long text.
func TestHelpTextConventions(t *testing.T) {
	walk(NewRoot(), func(cmd *cobra.Command) {
		if isGenerated(cmd) {
			return
		}
		path := cmd.CommandPath()
		if strings.Contains(cmd.Long, "Examples:") {
			t.Errorf("%s: examples belong in the Example field, not in Long", path)
		}
		if strings.TrimSpace(cmd.Long) == "" {
			t.Errorf("%s: missing Long help text", path)
		}
	})
}

// TestExamplesAreValid checks every line of every Example field against the
// real command tree: the command path must exist (no stale or misspelled
// subcommand) and each flag must be defined on that command. It catches an
// example left behind when a flag or command is renamed.
func TestExamplesAreValid(t *testing.T) {
	root := NewRoot()
	walk(root, func(cmd *cobra.Command) {
		for _, line := range strings.Split(cmd.Example, "\n") {
			line = strings.TrimSpace(line)
			if i := strings.Index(line, " #"); i >= 0 {
				line = strings.TrimSpace(line[:i])
			}
			// Only the xo invocation of a pipeline or redirection:
			// 'xo vm export <id> > vm.xva' ('<id>' placeholders have no spaces).
			for _, op := range []string{" | ", " > ", " < ", " && "} {
				if i := strings.Index(line, op); i >= 0 {
					line = strings.TrimSpace(line[:i])
				}
			}
			if !strings.HasPrefix(line, "xo ") {
				continue
			}
			checkExample(t, root, cmd.CommandPath(), line)
		}
	})
}

func checkExample(t *testing.T, root *cobra.Command, owner, line string) {
	t.Helper()
	args := splitArgs(strings.TrimPrefix(line, "xo "))
	target, rest, err := root.Find(args)
	if err != nil {
		t.Errorf("%s: example %q: %v", owner, line, err)
		return
	}
	var positional []string
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		name, _, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		var f *pflag.Flag
		if strings.HasPrefix(a, "--") {
			if f = target.Flags().Lookup(name); f == nil {
				f = target.InheritedFlags().Lookup(name)
			}
		} else {
			if f = target.Flags().ShorthandLookup(name[:1]); f == nil {
				f = target.InheritedFlags().ShorthandLookup(name[:1])
			}
		}
		if f == nil {
			t.Errorf("%s: example %q: unknown flag %q on %q", owner, line, a, target.CommandPath())
			continue
		}
		// A non-boolean flag given as '--flag value' consumes the next token.
		if !hasValue && f.NoOptDefVal == "" {
			i++
		}
	}
	if target.HasSubCommands() && len(positional) > 0 {
		t.Errorf("%s: example %q: %q is not a subcommand of %q", owner, line, positional[0], target.CommandPath())
	}
}

// splitArgs splits a command line on spaces, keeping single- and
// double-quoted strings together (quotes removed), like a shell would for
// the examples.
func splitArgs(s string) []string {
	var (
		args  []string
		cur   strings.Builder
		quote rune
		inArg bool
	)
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inArg = r, true
		case r == ' ' || r == '\t':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args
}

func walk(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, c := range cmd.Commands() {
		walk(c, fn)
	}
}

// isGenerated reports the commands cobra adds on its own (help, completion
// and the per-shell completion scripts), whose help text cobra owns.
func isGenerated(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "help" || c.Name() == "completion" {
			return true
		}
	}
	return false
}
