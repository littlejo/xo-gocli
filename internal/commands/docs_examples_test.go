package commands

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// repoRoot is the repository root, relative to this package.
const repoRoot = "../.."

// TestDocsCommandsAreValid checks every 'xo ...' command shown in a fenced
// code block of the Markdown documentation against the real command tree,
// like TestExamplesAreValid does for --help: a renamed command or flag
// cannot leave the README, the guides or the reference pages behind. The
// generated reference (docs/reference/cli) is checked by tools/gendocs.
func TestDocsCommandsAreValid(t *testing.T) {
	var files []string
	for _, name := range []string{"README.md", "CONTRIBUTING.md"} {
		if _, err := os.Stat(filepath.Join(repoRoot, name)); err == nil {
			files = append(files, filepath.Join(repoRoot, name))
		}
	}
	err := filepath.WalkDir(filepath.Join(repoRoot, "docs"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "cli" || d.Name() == "archive") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// The commands and flags cobra only adds at execution time.
	root := NewRoot()
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	walk(root, func(c *cobra.Command) { c.InitDefaultHelpFlag() })
	for _, file := range files {
		for _, line := range shellLines(t, file) {
			checkExample(t, root, file, line)
		}
	}
}

// shellLines returns the 'xo ...' invocations found in the shell code blocks
// of a Markdown file (fences tagged sh, bash, shell or console, or untagged),
// one per command: comments, prompts ('$ '),
// pipelines and redirections are stripped, continuation lines are joined,
// and commands after '&&', ';' or '$(' are extracted too.
func shellLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var (
		out     []string
		inFence bool
		isShell bool
		pending string
	)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		raw := sc.Text()
		if fence := strings.TrimSpace(raw); strings.HasPrefix(fence, "```") {
			inFence = !inFence
			switch strings.TrimPrefix(fence, "```") {
			case "", "sh", "bash", "shell", "console":
				isShell = true
			default:
				isShell = false
			}
			pending = ""
			continue
		}
		if !inFence || !isShell {
			continue
		}
		line := strings.TrimSpace(raw)
		if pending != "" {
			line = pending + " " + line
			pending = ""
		}
		if strings.HasSuffix(line, `\`) {
			pending = strings.TrimSuffix(line, `\`)
			continue
		}
		for _, cmd := range splitShell(line) {
			if strings.HasPrefix(cmd, "xo ") {
				out = append(out, cmd)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// splitShell cuts a shell line into the simple commands it runs, keeping
// only their xo-relevant part. It honours quotes, so a JMESPath query such
// as "[?a && b] | [0]" stays whole, and extracts the commands run inside
// $(...), even within double quotes.
func splitShell(line string) []string {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "$ "))
	if strings.HasPrefix(line, "#") {
		return nil
	}

	type level struct {
		buf   strings.Builder
		quote rune // the open quote at this level, or 0
	}
	var (
		raw   []string
		stack = []*level{{}}
	)
	emit := func(l *level) {
		raw = append(raw, l.buf.String())
		l.buf.Reset()
	}
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		r, cur := rs[i], stack[len(stack)-1]
		next := rune(0)
		if i+1 < len(rs) {
			next = rs[i+1]
		}
		switch {
		case cur.quote == '\'':
			if r == '\'' {
				cur.quote = 0
			}
			cur.buf.WriteRune(r)
		case r == '$' && next == '(':
			// Command substitution, also inside double quotes.
			cur.buf.WriteString("X")
			stack = append(stack, &level{})
			i++
		case r == ')' && cur.quote == 0 && len(stack) > 1:
			emit(cur)
			stack = stack[:len(stack)-1]
		case cur.quote == '"':
			if r == '"' {
				cur.quote = 0
			}
			cur.buf.WriteRune(r)
		case r == '\'' || r == '"':
			cur.quote = r
			cur.buf.WriteRune(r)
		case r == '#' && (i == 0 || rs[i-1] == ' '):
			i = len(rs) // a comment ends the line
		case r == ';' || r == '|' || r == '&':
			if next == r {
				i++ // && or ||
			}
			emit(cur)
		default:
			cur.buf.WriteRune(r)
		}
	}
	for len(stack) > 0 {
		emit(stack[len(stack)-1])
		stack = stack[:len(stack)-1]
	}

	var cmds []string
	for _, part := range raw {
		part = strings.TrimSpace(cutRedirection(part))
		// Leading environment assignments: 'XOA_TIMEOUT=5m xo vm export …'.
		for {
			f := strings.Fields(part)
			if len(f) == 0 || !strings.Contains(f[0], "=") || strings.HasPrefix(f[0], "-") {
				break
			}
			part = strings.TrimSpace(strings.TrimPrefix(part, f[0]))
		}
		for _, wrapper := range []string{"sudo ", "exec ", "time ", "if ", "! ", "then ", "do ", "[ "} {
			part = strings.TrimPrefix(part, wrapper)
		}
		if strings.HasPrefix(part, "xo ") {
			cmds = append(cmds, strings.TrimSpace(part))
		}
	}
	return cmds
}

// cutRedirection drops an unquoted redirection and what follows it
// ('> vm.xva', '2>/dev/null', '< in.xva'), but keeps placeholders such as
// '<id>', whose angle brackets are not separated by spaces.
func cutRedirection(cmd string) string {
	var quote rune
	rs := []rune(cmd)
	for i, r := range rs {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '>' && i > 0 && (rs[i-1] == ' ' || (rs[i-1] == '2' && i > 1 && rs[i-2] == ' ')):
			return string(rs[:i-1])
		case r == '<' && i > 0 && rs[i-1] == ' ' && i+1 < len(rs) && rs[i+1] == ' ':
			return string(rs[:i])
		}
	}
	return cmd
}

// TestSplitShell pins the shell parsing the docs check relies on.
func TestSplitShell(t *testing.T) {
	for in, want := range map[string][]string{
		`xo vm list --query "[?a && b].id | [0]" --output text`:                                  {`xo vm list --query "[?a && b].id | [0]" --output text`},
		`if [ "$(xo vm get "$VM" --query 'power_state' --output text)" = "Running" ]; then`:      {`xo vm get "$VM" --query 'power_state' --output text`},
		`NEW=$(xo vm import "b-$(date +%F).xva" --pool "$P" --output json | jq -r '.vm')`:        {`xo vm import "b-X.xva" --pool "$P" --output json`},
		`xo vm export <id> > vm.xva   # export`:                                                  {`xo vm export <id>`},
		`XOA_TIMEOUT=5m xo vm export <id> --file vm.xva 2>/dev/null && echo ok`:                  {`xo vm export <id> --file vm.xva`},
		`xo --profile "$p" vm list --power-state Running --query 'length(@)' || echo "(failed)"`: {`xo --profile "$p" vm list --power-state Running --query 'length(@)'`},
		`xo vm import - --pool <pool> < vm.xva`:                                                  {`xo vm import - --pool <pool>`},
	} {
		if got := splitShell(in); !reflect.DeepEqual(got, want) {
			t.Errorf("splitShell(%q)\n got  %q\n want %q", in, got, want)
		}
	}
}
