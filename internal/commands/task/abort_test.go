package task

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
)

type abortRequest struct {
	method string
	path   string
}

// abortServer serves the abort flow: a GET /rest/v0/tasks/{id} returning
// fixture (404 when fixture is empty) and a POST /rest/v0/tasks/{id}/abort,
// both behind the authenticationToken cookie. Every request is recorded so
// the tests can assert whether the abort endpoint was contacted.
type abortServer struct {
	*httptest.Server
	requests []abortRequest
}

func newAbortServer(t *testing.T, fixture string, abortResponse int) *abortServer {
	t.Helper()
	s := &abortServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests = append(s.requests, abortRequest{method: r.Method, path: r.URL.Path})

		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/tasks/"):
			if fixture == "" {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"not found"}`)
				return
			}
			_, _ = fmt.Fprint(w, fixture)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/abort"):
			w.WriteHeader(abortResponse)
			_, _ = fmt.Fprint(w, `{"success":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

// post reports the POST request, if any, recorded by the server.
func (s *abortServer) post() (abortRequest, bool) {
	for _, r := range s.requests {
		if r.method == http.MethodPost {
			return r, true
		}
	}
	return abortRequest{}, false
}

func newAbortTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	root.AddCommand(newAbortCommand())
	return root
}

func runAbortCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newAbortTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestTaskAbort(t *testing.T) {
	server := newAbortServer(t, fixtureTaskPending, http.StatusOK)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runAbortCmd(t, "abort", "p1", "--yes")
	if err != nil {
		t.Fatalf("task abort: %v", err)
	}
	if !strings.Contains(out, "Aborted task p1") {
		t.Fatalf("unexpected output: %s", out)
	}

	post, ok := server.post()
	if !ok {
		t.Fatal("expected a POST to the abort endpoint")
	}
	if post.path != "/rest/v0/tasks/p1/abort" {
		t.Fatalf("unexpected abort path: %s", post.path)
	}
}

func TestTaskAbortJSON(t *testing.T) {
	server := newAbortServer(t, fixtureTaskPending, http.StatusOK)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runAbortCmd(t, "abort", "p1", "--yes", "--output", "json")
	if err != nil {
		t.Fatalf("task abort --output json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if doc["action"] != "abort" || doc["task"] != "p1" {
		t.Fatalf("unexpected JSON document: %s", out)
	}
}

// Aborting asks for confirmation; without --yes and on a non-terminal stdin
// the command must refuse before contacting the abort endpoint.
func TestTaskAbortRequiresConfirmation(t *testing.T) {
	server := newAbortServer(t, fixtureTaskPending, http.StatusOK)
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runAbortCmd(t, "abort", "p1")
	if err == nil {
		t.Fatal("expected an error without --yes and a non-terminal stdin")
	}
	if !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := server.post(); ok {
		t.Fatal("the abort endpoint must not be contacted without a confirmation")
	}
}

// $XOA_YES is the script counterpart of --yes: it must skip the confirmation.
func TestTaskAbortEnvYes(t *testing.T) {
	server := newAbortServer(t, fixtureTaskPending, http.StatusOK)
	defer server.Close()
	isolatePointers(t, server.URL)
	t.Setenv("XOA_YES", "1")

	out, err := runAbortCmd(t, "abort", "p1")
	if err != nil {
		t.Fatalf("task abort with XOA_YES=1: %v", err)
	}
	if !strings.Contains(out, "Aborted task p1") {
		t.Fatalf("unexpected output: %s", out)
	}
}

// The pre-check must reject aborting a task that already reached a terminal
// state, with a clear error and without contacting the abort endpoint.
func TestTaskAbortAlreadyTerminal(t *testing.T) {
	for _, status := range []string{"success", "failure", "interrupted"} {
		t.Run(status, func(t *testing.T) {
			fixture := strings.Replace(fixtureTaskPending, `"pending"`, `"`+status+`"`, 1)
			server := newAbortServer(t, fixture, http.StatusOK)
			defer server.Close()
			isolatePointers(t, server.URL)

			_, err := runAbortCmd(t, "abort", "p1", "--yes")
			if err == nil {
				t.Fatalf("expected an error when the task is already %s", status)
			}
			if !strings.Contains(err.Error(), "cannot be aborted: it is already "+status) {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, ok := server.post(); ok {
				t.Fatal("the abort endpoint must not be contacted for a finished task")
			}
		})
	}
}

// A missing task is reported before any abort is attempted.
func TestTaskAbortNotFound(t *testing.T) {
	server := newAbortServer(t, "", http.StatusOK)
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runAbortCmd(t, "abort", "p1", "--yes")
	if err == nil {
		t.Fatal("expected an error when the task does not exist")
	}
	if !strings.Contains(err.Error(), `task "p1" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := server.post(); ok {
		t.Fatal("the abort endpoint must not be contacted for a missing task")
	}
}

// An id with a path segment is rejected before any request is made.
func TestTaskAbortBadID(t *testing.T) {
	server := newAbortServer(t, fixtureTaskPending, http.StatusOK)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runAbortCmd(t, "abort", "a/b"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
	if len(server.requests) != 0 {
		t.Fatalf("no request must be made for an invalid id: %v", server.requests)
	}
}

// When the API rejects the abort, the error is concise and keeps the task id.
func TestTaskAbortAPIError(t *testing.T) {
	for name, abortStatus := range map[string]int{
		"bad-request": http.StatusBadRequest,
		"server":      http.StatusInternalServerError,
	} {
		t.Run(name, func(t *testing.T) {
			server := newAbortServer(t, fixtureTaskPending, abortStatus)
			defer server.Close()
			isolatePointers(t, server.URL)

			_, err := runAbortCmd(t, "abort", "p1", "--yes")
			if err == nil {
				t.Fatal("expected an error when the API rejects the abort")
			}
			if !strings.Contains(err.Error(), `cannot abort task "p1"`) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTaskLabel(t *testing.T) {
	full := map[string]any{
		"id": "p1",
		"properties": map[string]any{
			"name": "clean_shutdown",
			"type": "VM",
		},
	}
	if got := taskLabel(full); got != "p1 (VM clean_shutdown)" {
		t.Fatalf("unexpected label: %s", got)
	}

	bare := map[string]any{"id": "p1"}
	if got := taskLabel(bare); got != "p1" {
		t.Fatalf("unexpected label: %s", got)
	}

	nameOnly := map[string]any{
		"id":         "p1",
		"properties": map[string]any{"name": "clean_shutdown"},
	}
	if got := taskLabel(nameOnly); got != "p1 (clean_shutdown)" {
		t.Fatalf("unexpected label: %s", got)
	}

	typeOnly := map[string]any{
		"id":         "p1",
		"properties": map[string]any{"type": "VM"},
	}
	if got := taskLabel(typeOnly); got != "p1 (VM)" {
		t.Fatalf("unexpected label: %s", got)
	}
}
