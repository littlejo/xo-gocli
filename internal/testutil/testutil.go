// Package testutil holds the helpers the command tests share: a root command
// with the real global flags, a runner that captures stdout and stderr, and
// the authentication check of the fake Xen Orchestra servers. It is imported
// by _test.go files only.
package testutil

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// Token is the token testenv.Isolate configures, and the only one the fake
// servers accept.
const Token = "test-token"

// Root returns an 'xo' root command with the global flags (cli.AddGlobalFlags)
// holding cmds, configured like the real one: no usage or error printing, so
// tests see only the command's own output.
func Root(cmds ...*cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cli.AddGlobalFlags(root)
	root.AddCommand(cmds...)
	return root
}

// Execute runs root with args and returns what it wrote to stdout and to
// stderr.
func Execute(root *cobra.Command, args ...string) (stdout, stderr string, err error) {
	var out, errOut strings.Builder
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err = root.ExecuteContext(context.Background())
	return out.String(), errOut.String(), err
}

// ExecuteCombined runs root with args and returns stdout and stderr written
// to one buffer, in the order the command wrote them.
func ExecuteCombined(root *cobra.Command, args ...string) (string, error) {
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

// Authorized reports whether r carries the test token, as Xen Orchestra
// checks the authenticationToken cookie. When it does not, it answers 401
// like the API does; the handler then returns.
func Authorized(w http.ResponseWriter, r *http.Request) bool {
	if cookie, err := r.Cookie("authenticationToken"); err == nil && cookie.Value == Token {
		return true
	}
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
	return false
}
