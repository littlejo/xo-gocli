package task

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/taskwait"
)

const fixtureTaskPending = `{
	"id": "p1",
	"status": "pending",
	"start": "2026-09-28T10:05:00.000Z",
	"properties": {"name": "start", "type": "VM"}
}`

const fixtureTaskSuccess = `{
	"id": "s1",
	"status": "success",
	"start": "2026-09-28T10:05:00.000Z",
	"end": "2026-09-28T10:05:02.000Z",
	"properties": {"name": "start", "type": "VM"},
	"result": {"code": "OK", "message": "done"}
}`

const fixtureTaskSuccessStringResult = `{
	"id": "s2",
	"status": "success",
	"start": 1700000000000,
	"end": 1700000001000,
	"properties": {"name": "provisioning", "type": "Pool"},
	"result": "a plain string result"
}`

const fixtureTaskInterrupted = `{
	"id": "i1",
	"status": "interrupted",
	"start": "2026-09-28T10:05:00.000Z",
	"end": "2026-09-28T10:05:01.000Z",
	"properties": {"name": "reboot", "type": "VM"}
}`

// fakeWaitXO serves GET /rest/v0/tasks/{id} with a per-test handler that also
// receives the number of previous calls, so tests can script "pending at first,
// then success" sequences. The handler owns the response.
func fakeWaitXO(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, calls *int)) *httptest.Server {
	t.Helper()
	var calls int
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/v0/tasks/") {
			http.NotFound(w, r)
			return
		}
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		handler(w, r, &calls)
	}))
}

func newWaitTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	root.AddCommand(newWaitCommand())
	return root
}

// runWaitCmd executes the wait command with stdout and stderr captured
// separately: the completed task goes to stdout, the "Waiting for task ..."
// progress line goes to stderr.
func runWaitCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newWaitTestRoot()
	var out, errb strings.Builder
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err = root.ExecuteContext(context.Background())
	return out.String(), errb.String(), err
}

// shortenPoll makes the wait loop poll quickly instead of every 2 seconds, so
// the tests do not sleep for real. It restores the original value afterwards.
func shortenPoll(t *testing.T, d time.Duration) {
	t.Helper()
	old := taskwait.PollInterval
	taskwait.PollInterval = d
	t.Cleanup(func() { taskwait.PollInterval = old })
}

func TestTaskWaitSuccess(t *testing.T) {
	server := fakeWaitXO(t, func(w http.ResponseWriter, _ *http.Request, calls *int) {
		w.Header().Set("Content-Type", "application/json")
		if *calls == 0 {
			_, _ = fmt.Fprint(w, fixtureTaskPending)
		} else {
			_, _ = fmt.Fprint(w, fixtureTaskSuccess)
		}
		*calls++
	})
	defer server.Close()
	isolatePointers(t, server.URL)
	shortenPoll(t, time.Millisecond)

	out, stderr, err := runWaitCmd(t, "wait", "s1")
	if err != nil {
		t.Fatalf("task wait: %v", err)
	}
	if !strings.Contains(stderr, "Waiting for task s1 to complete") {
		t.Fatalf("expected a progress hint on stderr:\n%s", stderr)
	}
	for _, expected := range []string{"ID", "STATUS", "s1", "success", "VM", "start"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
}

func TestTaskWaitFailure(t *testing.T) {
	// fixtureTask (in get_test.go) is a failed task whose result carries a
	// message; wait must render it and exit non-zero with that message.
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, _, err := runWaitCmd(t, "wait", "f6e5d4c3b2a1")
	if err == nil {
		t.Fatal("expected a non-zero exit when the task fails")
	}
	if !strings.Contains(err.Error(), "task f6e5d4c3b2a1 failed: VM not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "failure") {
		t.Fatalf("the completed task must still be rendered:\n%s", out)
	}
}

func TestTaskWaitInterrupted(t *testing.T) {
	server := fakeWaitXO(t, func(w http.ResponseWriter, _ *http.Request, _ *int) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTaskInterrupted)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, _, err := runWaitCmd(t, "wait", "i1")
	if err == nil {
		t.Fatal("expected a non-zero exit when the task is interrupted")
	}
	if !strings.Contains(err.Error(), "task i1 was interrupted") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Xen Orchestra sometimes returns a plain string as the task "result". The SDK's
// typed Task().Wait cannot unmarshal such a task, so wait must go through the
// raw REST path (like 'xo task get') and still succeed on it.
func TestTaskWaitStringResult(t *testing.T) {
	server := fakeWaitXO(t, func(w http.ResponseWriter, _ *http.Request, _ *int) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTaskSuccessStringResult)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	out, _, err := runWaitCmd(t, "wait", "s2", "--output", "json")
	if err != nil {
		t.Fatalf("task wait on a string-result task: %v", err)
	}
	var task map[string]any
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if task["result"] != "a plain string result" {
		t.Fatalf("expected the string result to pass through, got %v", task["result"])
	}
}

func TestTaskWaitTimeout(t *testing.T) {
	server := fakeWaitXO(t, func(w http.ResponseWriter, _ *http.Request, _ *int) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTaskPending)
	})
	defer server.Close()
	isolatePointers(t, server.URL)
	shortenPoll(t, 5*time.Millisecond)

	_, _, err := runWaitCmd(t, "wait", "p1", "--timeout", "150ms")
	if err == nil {
		t.Fatal("expected a timeout error when the task does not complete")
	}
	if !strings.Contains(err.Error(), `task p1 did not complete within 150ms`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskWaitNotFound(t *testing.T) {
	server := fakeWaitXO(t, func(w http.ResponseWriter, _ *http.Request, _ *int) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"not found"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, _, err := runWaitCmd(t, "wait", "nope")
	if err == nil {
		t.Fatal("expected an error when the task does not exist")
	}
	if !strings.Contains(err.Error(), `task "nope" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskWaitBadID(t *testing.T) {
	server := fakeWaitXO(t, func(w http.ResponseWriter, _ *http.Request, _ *int) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTaskSuccess)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, _, err := runWaitCmd(t, "wait", "a/b"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
}

func TestTaskWaitJSON(t *testing.T) {
	server := fakeWaitXO(t, func(w http.ResponseWriter, _ *http.Request, _ *int) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTaskSuccess)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	out, _, err := runWaitCmd(t, "wait", "s1", "--output", "json")
	if err != nil {
		t.Fatalf("task wait --output json: %v", err)
	}
	var task map[string]any
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if task["id"] != "s1" || task["status"] != "success" {
		t.Fatalf("unexpected task payload: %s", out)
	}
}

func TestTaskWaitQuery(t *testing.T) {
	server := fakeWaitXO(t, func(w http.ResponseWriter, _ *http.Request, _ *int) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTaskSuccess)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	out, _, err := runWaitCmd(t, "wait", "s1", "--query", "status")
	if err != nil {
		t.Fatalf("task wait --query: %v", err)
	}
	if strings.TrimSpace(out) != "success" {
		t.Fatalf("unexpected query output: %q", out)
	}
}
