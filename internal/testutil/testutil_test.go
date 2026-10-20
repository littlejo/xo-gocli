package testutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

func TestAuthorized(t *testing.T) {
	for _, tc := range []struct {
		token string
		ok    bool
		code  int
	}{{Token, true, http.StatusOK}, {"wrong", false, http.StatusUnauthorized}, {"", false, http.StatusUnauthorized}} {
		r := httptest.NewRequest(http.MethodGet, "/rest/v0/vms", nil)
		if tc.token != "" {
			r.AddCookie(&http.Cookie{Name: "authenticationToken", Value: tc.token})
		}
		w := httptest.NewRecorder()
		if got := Authorized(w, r); got != tc.ok || w.Code != tc.code {
			t.Fatalf("token %q: Authorized = %v (status %d), want %v (%d)", tc.token, got, w.Code, tc.ok, tc.code)
		}
	}
}

func TestRootParsesGlobalFlags(t *testing.T) {
	sub := &cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, _ []string) error {
		out, _ := cmd.Flags().GetString("output")
		_, err := fmt.Fprint(cmd.OutOrStdout(), out)
		_, _ = fmt.Fprint(cmd.ErrOrStderr(), "diag")
		return err
	}}
	stdout, stderr, err := Execute(Root(sub), "probe", "-o", "json", "--debug")
	if err != nil || stdout != "json" || stderr != "diag" {
		t.Fatalf("Execute = %q, %q, %v", stdout, stderr, err)
	}
}
