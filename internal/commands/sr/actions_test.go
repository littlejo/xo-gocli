package sr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
)

const fixtureTask = `{
	"id": "task-123",
	"status": "success",
	"start": 1700000000000,
	"end": 1700000001000
}`

// actionServer is a fake XO server for the per-SR action endpoints. It
// records every request so tests can assert on method and path, and answers:
//
//	GET  /rest/v0/srs/{id}                -> fixtureSR
//	GET  /rest/v0/tasks/{id}              -> fixtureTask (success)
//	POST /rest/v0/srs/{id}/actions/scan          -> {"taskId":"task-123"}
//	POST /rest/v0/srs/{id}/actions/reclaim_space -> {"taskId":"task-123"}
type actionServer struct {
	*httptest.Server
	requests []actionRequest
}

type actionRequest struct {
	Method string
	Path   string
}

func newActionServer(t *testing.T) *actionServer {
	t.Helper()
	s := &actionServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests = append(s.requests, actionRequest{Method: r.Method, Path: r.URL.Path})

		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/tasks/"):
			_, _ = fmt.Fprint(w, fixtureTask)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/srs/"):
			_, _ = fmt.Fprint(w, fixtureSR)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/actions/"):
			_, _ = fmt.Fprint(w, `{"taskId":"task-123"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func (s *actionServer) post() (actionRequest, bool) {
	for _, r := range s.requests {
		if r.Method == http.MethodPost {
			return r, true
		}
	}
	return actionRequest{}, false
}

func newActionTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	root.AddCommand(NewCommand())
	return root
}

func runSRAction(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newActionTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestSRScan(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runSRAction(t, "sr", "scan", "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatalf("sr scan: %v", err)
	}
	if !strings.Contains(out, "scan") || !strings.Contains(out, "Local storage") || !strings.Contains(out, "task-123") {
		t.Fatalf("unexpected scan output: %q", out)
	}
	req, ok := server.post()
	if !ok {
		t.Fatal("expected a POST scan request")
	}
	if req.Path != "/rest/v0/srs/11111111-1111-4111-8111-111111111111/actions/scan" {
		t.Fatalf("unexpected scan path: %s", req.Path)
	}
}

func TestSRReclaimSpace(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runSRAction(t, "sr", "reclaim-space", "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatalf("sr reclaim-space: %v", err)
	}
	if !strings.Contains(out, "reclaim space") || !strings.Contains(out, "Local storage") || !strings.Contains(out, "task-123") {
		t.Fatalf("unexpected reclaim-space output: %q", out)
	}
	req, ok := server.post()
	if !ok {
		t.Fatal("expected a POST reclaim_space request")
	}
	if req.Path != "/rest/v0/srs/11111111-1111-4111-8111-111111111111/actions/reclaim_space" {
		t.Fatalf("unexpected reclaim_space path: %s", req.Path)
	}
}

func TestSRScanNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rest/v0/srs/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runSRAction(t, "sr", "scan", "11111111-1111-4111-8111-111111111111")
	if err == nil {
		t.Fatal("expected an error when the SR does not exist")
	}
	if !strings.Contains(err.Error(), `SR "11111111-1111-4111-8111-111111111111" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSRScanBadID(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runSRAction(t, "sr", "scan", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
	if _, ok := server.post(); ok {
		t.Fatal("no scan request must be sent for an invalid id")
	}
}

func TestSRReclaimSpaceAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rest/v0/srs/") {
			_, _ = fmt.Fprint(w, fixtureSR)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"boom"}`)
	}))
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runSRAction(t, "sr", "reclaim-space", "11111111-1111-4111-8111-111111111111")
	if err == nil {
		t.Fatal("expected an error when the action fails")
	}
	if !strings.Contains(err.Error(), "cannot reclaim space") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- --wait ------------------------------------------------------------------

func TestSRScanWait(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runSRAction(t, "sr", "scan", "11111111-1111-4111-8111-111111111111", "--wait")
	if err != nil {
		t.Fatalf("sr scan --wait: %v", err)
	}
	// The completed task is rendered instead of the "Requested scan" line.
	for _, expected := range []string{"ID", "STATUS", "task-123", "success"} {
		if !strings.Contains(out, expected) {
			t.Errorf("wait output missing %q:\n%s", expected, out)
		}
	}
	if strings.Contains(out, "Requested scan") {
		t.Fatalf("the plain action line must be replaced by the task when --wait is set:\n%s", out)
	}
}
