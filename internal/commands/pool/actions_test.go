package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// fixtureTaskSuccess is a finished task as served by GET /rest/v0/tasks/{id}.
// The SDK's task poller returns as soon as the status is success or failure,
// so no real wait happens in tests.
const fixtureTaskSuccess = `{
	"id": "task-123",
	"status": "success",
	"start": 1700000000000,
	"end": 1700000001000
}`

const fixtureTaskFailure = `{
	"id": "task-123",
	"status": "failure",
	"start": 1700000000000,
	"end": 1700000001000,
	"result": {"message": "no available host"}
}`

// fixtureTaskInterrupted is a task the server interrupted. It is the case the
// SDK's own task.Wait never treats as terminal (it only stops on success or
// failure), so an action run against it would block until the context dies:
// the tests exercise it with a --timeout to prove the wait is bounded.
const fixtureTaskInterrupted = `{
	"id": "task-123",
	"status": "interrupted",
	"start": 1700000000000,
	"end": 1700000001000
}`

const (
	testPoolID = "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	testTaskID = "task-123"
)

// actionServer is a fake XO server for the pool action endpoints. It records
// every request so tests can assert on the method, path and body, and answers:
//
//	GET  /rest/v0/pools/{id}            -> fixturePool
//	GET  /rest/v0/tasks/{id}            -> task fixture (status configurable)
//	POST /rest/v0/pools/{id}/actions/*  -> {"taskId":"task-123"}
type actionServer struct {
	*httptest.Server
	t            *testing.T
	requests     []request
	poolStatus   int    // status code for GET /rest/v0/pools/{id}
	taskStatus   string // status of the task returned by GET /rest/v0/tasks/{id}
	actionStatus int    // status code for the POST action endpoint
}

type request struct {
	Method string
	Path   string
	Body   string
}

func newActionServer(t *testing.T) *actionServer {
	t.Helper()
	s := &actionServer{t: t, poolStatus: http.StatusOK, taskStatus: "success"}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := readActionBody(r)
		s.requests = append(s.requests, request{Method: r.Method, Path: r.URL.Path, Body: body})

		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/pools/"):
			if s.poolStatus != http.StatusOK {
				w.WriteHeader(s.poolStatus)
				_, _ = fmt.Fprint(w, `{"message":"not found"}`)
				return
			}
			_, _ = fmt.Fprint(w, fixturePool)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/tasks/"):
			switch s.taskStatus {
			case "failure":
				_, _ = fmt.Fprint(w, fixtureTaskFailure)
			case "interrupted":
				_, _ = fmt.Fprint(w, fixtureTaskInterrupted)
			default:
				_, _ = fmt.Fprint(w, fixtureTaskSuccess)
			}
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/actions/"):
			if s.actionStatus != 0 {
				w.WriteHeader(s.actionStatus)
				_, _ = fmt.Fprint(w, `{"message":"boom"}`)
				return
			}
			_, _ = fmt.Fprintf(w, `{"taskId":"%s"}`, testTaskID)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func (s *actionServer) post() (request, bool) {
	for _, r := range s.requests {
		if r.Method == http.MethodPost {
			return r, true
		}
	}
	return request{}, false
}

func readActionBody(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	data, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	return string(data), err
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
	root.AddCommand(newRollingUpdateCommand())
	root.AddCommand(newRollingRebootCommand())
	root.AddCommand(newEmergencyShutdownCommand())
	return root
}

// runActionTest executes the command with an explicit non-terminal stdin so
// destructive commands that lack --yes fail deterministically instead of
// blocking on a prompt (even when the test itself runs under a terminal). It
// returns stdout and stderr separately: progress hints are written to stderr
// so stdout stays clean for scripting, and tests assert on each stream on its
// own.
func runActionTest(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newActionTestRoot()
	root.SetIn(strings.NewReader(""))
	var out, errb strings.Builder
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err = root.ExecuteContext(context.Background())
	return out.String(), errb.String(), err
}

// --- rolling-update ---------------------------------------------------------

func TestPoolRollingUpdate(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, _, err := runActionTest(t, "rolling-update", testPoolID)
	if err != nil {
		t.Fatalf("pool rolling-update: %v", err)
	}
	if !strings.Contains(out, "Pool prod") || !strings.Contains(out, "rolling update applied") {
		t.Fatalf("unexpected rolling-update output: %q", out)
	}
	req, ok := server.post()
	if !ok {
		t.Fatal("expected a POST action request")
	}
	if req.Path != "/rest/v0/pools/"+testPoolID+"/actions/rolling_update" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

// rolling-update reboots the hosts one by one without ever taking the pool
// down, so unlike the other two actions it must run without confirmation even
// when stdin is not a terminal.
func TestPoolRollingUpdateRequiresNoConfirmation(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, _, err := runActionTest(t, "rolling-update", testPoolID); err != nil {
		t.Fatalf("rolling-update must not require confirmation: %v", err)
	}
}

func TestPoolRollingUpdateJSON(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, _, err := runActionTest(t, "rolling-update", testPoolID, "--output", "json")
	if err != nil {
		t.Fatalf("pool rolling-update --output json: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if result["action"] != "rolling update" || result["pool"] != "Pool prod" || result["status"] != "done" {
		t.Fatalf("unexpected rolling-update payload: %s", out)
	}
}

// A backing task that the server interrupted is never terminal for the SDK's
// own wait loop (it stops only on success/failure), so without a bound the
// command would block until the context dies. With --timeout the wait must
// end with a concise deadline error instead (S3: bound the unbounded waits).
func TestPoolRollingUpdateInterruptedTaskIsBoundedByTimeout(t *testing.T) {
	server := newActionServer(t)
	server.taskStatus = "interrupted"
	defer server.Close()
	isolatePointers(t, server.URL)

	_, _, err := runActionTest(t, "rolling-update", testPoolID, "--timeout", "1s")
	if err == nil {
		t.Fatal("expected a timeout error for an interrupted backing task")
	}
	if !strings.Contains(err.Error(), "did not complete within 1s") {
		t.Fatalf("expected a concise deadline error, got: %v", err)
	}
	if strings.Contains(err.Error(), "task wait timed out") {
		t.Fatalf("the raw SDK error must not leak, got: %v", err)
	}
}

// --- rolling-reboot ---------------------------------------------------------

func TestPoolRollingReboot(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, _, err := runActionTest(t, "rolling-reboot", testPoolID, "--yes")
	if err != nil {
		t.Fatalf("pool rolling-reboot: %v", err)
	}
	if !strings.Contains(out, "Pool prod") || !strings.Contains(out, "rolling reboot done") {
		t.Fatalf("unexpected rolling-reboot output: %q", out)
	}
	req, ok := server.post()
	if !ok {
		t.Fatal("expected a POST action request")
	}
	if req.Path != "/rest/v0/pools/"+testPoolID+"/actions/rolling_reboot" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

func TestPoolRollingRebootRequiresConfirmation(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	_, _, err := runActionTest(t, "rolling-reboot", testPoolID)
	if err == nil {
		t.Fatal("expected an error when no confirmation is given and stdin is not a terminal")
	}
	if _, ok := server.post(); ok {
		t.Fatal("the action must not be executed without confirmation")
	}
}

// $XOA_YES is the script counterpart of --yes: it skips the confirmation
// without a terminal.
func TestPoolRollingRebootSkipsConfirmationWithEnvYes(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)
	t.Setenv("XOA_YES", "1")

	if _, _, err := runActionTest(t, "rolling-reboot", testPoolID); err != nil {
		t.Fatalf("rolling-reboot with XOA_YES=1: %v", err)
	}
	req, _ := server.post()
	if req.Path != "/rest/v0/pools/"+testPoolID+"/actions/rolling_reboot" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

// --- emergency-shutdown -----------------------------------------------------

func TestPoolEmergencyShutdown(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, _, err := runActionTest(t, "emergency-shutdown", testPoolID, "--yes")
	if err != nil {
		t.Fatalf("pool emergency-shutdown: %v", err)
	}
	if !strings.Contains(out, "Pool prod") || !strings.Contains(out, "emergency shutdown done") {
		t.Fatalf("unexpected emergency-shutdown output: %q", out)
	}
	req, ok := server.post()
	if !ok {
		t.Fatal("expected a POST action request")
	}
	if req.Path != "/rest/v0/pools/"+testPoolID+"/actions/emergency_shutdown" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

func TestPoolEmergencyShutdownRequiresConfirmation(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	_, _, err := runActionTest(t, "emergency-shutdown", testPoolID)
	if err == nil {
		t.Fatal("expected an error when no confirmation is given and stdin is not a terminal")
	}
	if _, ok := server.post(); ok {
		t.Fatal("the action must not be executed without confirmation")
	}
}

// --- shared error handling --------------------------------------------------

func TestPoolActionAPIError(t *testing.T) {
	server := newActionServer(t)
	server.actionStatus = http.StatusInternalServerError
	defer server.Close()
	isolatePointers(t, server.URL)

	_, _, err := runActionTest(t, "rolling-update", testPoolID)
	if err == nil {
		t.Fatal("expected an error when the API fails")
	}
	if !strings.Contains(err.Error(), `cannot rolling update pool "Pool prod"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoolActionTaskFailure(t *testing.T) {
	server := newActionServer(t)
	server.taskStatus = "failure"
	defer server.Close()
	isolatePointers(t, server.URL)

	_, _, err := runActionTest(t, "rolling-reboot", testPoolID, "--yes")
	if err == nil {
		t.Fatal("expected an error when the backing task fails")
	}
	if !strings.Contains(err.Error(), `cannot rolling reboot pool "Pool prod"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "no available host") {
		t.Fatalf("task failure message must be surfaced: %v", err)
	}
}

func TestPoolActionNotFound(t *testing.T) {
	server := newActionServer(t)
	server.poolStatus = http.StatusNotFound
	defer server.Close()
	isolatePointers(t, server.URL)

	_, _, err := runActionTest(t, "rolling-update", testPoolID)
	if err == nil {
		t.Fatal("expected an error when the pool does not exist")
	}
	if !strings.Contains(err.Error(), `pool "aaaaaaaa-bbbb-cccc-dddd-000000000001" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := server.post(); ok {
		t.Fatal("the action must not be executed for a missing pool")
	}
}

func TestPoolActionBadID(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, _, err := runActionTest(t, "rolling-update", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
}
