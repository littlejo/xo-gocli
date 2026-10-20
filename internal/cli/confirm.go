package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// IsTerminal reports whether in is connected to a terminal.
func IsTerminal(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

// Confirm asks the user to confirm a destructive operation. When yes is set
// (--yes or $XOA_YES) it always proceeds; when stdin is not a terminal it
// refuses so that automation never blocks, forcing the caller to pass --yes
// explicitly. The prompt goes to stderr so stdout keeps only the data.
func Confirm(cmd *cobra.Command, message string, yes bool) (bool, error) {
	if yes {
		return true, nil
	}
	in := cmd.InOrStdin()
	if !IsTerminal(in) {
		return false, fmt.Errorf("confirmation required: re-run with --yes to proceed non-interactively")
	}
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N]: ", message); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	if err != nil && answer == "" {
		return false, fmt.Errorf("cannot read confirmation: %w", err)
	}
	return answer == "y" || answer == "yes", nil
}
