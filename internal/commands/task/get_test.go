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
	"github.com/littlejo/xo-gocli/internal/output"
)

const fixtureTask = `{
	"id": "f6e5d4c3b2a1",
	"status": "failure",
	"start": "2026-09-28T10:05:00.000Z",
	"end": "2026-09-28T10:05:02.000Z",
	"properties": {
		"name": "clean_shutdown",
		"type": "VM",
		"objectId": "550e8400-e29b-41d4-a716-446655440002"
	},
	"result": {
		"code": "VM_NOT_FOUND",
		"message": "VM not found"
	}
}`

const fixtureTaskStringResult = `{
	"id": "888",
	"status": "success",
	"start": 1700000000000,
	"end": "2026-09-28T11:00:00.000Z",
	"properties": {
		"name": "provisioning",
		"type": "Pool"
	},
	"result": "a plain string result",
	"tasks": [
		{
			"id": "child-1",
			"status": "success",
			"result": "nested string result"
		}
	]
}`

// fakeXOGet serves GET /rest/v0/tasks/{id}.
func fakeXOGet(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/v0/tasks/") {
			http.NotFound(w, r)
			return
		}
		if handler != nil {
			handler(w, r)
			return
		}
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTask)
	}))
}

func newGetTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	root.AddCommand(newGetCommand())
	return root
}

func runGet(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newGetTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

// TestTaskGetDetail checks the human view of `task get`: it is a detail sheet,
// not the one-row table shared with `task list`. The fixture is a failed task,
// so the error (code + message) is shown and the duration is computed.
func TestTaskGetDetail(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "f6e5d4c3b2a1")
	if err != nil {
		t.Fatalf("task get: %v", err)
	}
	for _, expected := range []string{
		"Task f6e5d4c3b2a1  (failure)",
		output.DetailField("Type", "VM"),
		output.DetailField("Name", "clean_shutdown"),
		output.DetailField("Target", "550e8400-e29b-41d4-a716-446655440002"),
		output.DetailField("Started", "2026-09-28T10:05:00Z"),
		output.DetailField("Ended", "2026-09-28T10:05:02Z"),
		output.DetailField("Duration", "2s"),
		output.DetailField("Error", "VM_NOT_FOUND — VM not found"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

func TestTaskGetJSON(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "f6e5d4c3b2a1", "--output", "json")
	if err != nil {
		t.Fatalf("task get --output json: %v", err)
	}

	var task map[string]any
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if task["id"] != "f6e5d4c3b2a1" || task["status"] != "failure" {
		t.Fatalf("unexpected task payload: %s", out)
	}
	result, _ := task["result"].(map[string]any)
	if result["message"] != "VM not found" {
		t.Fatalf("expected result.message to be preserved, got %v", task["result"])
	}
}

// TestTaskGetStringResult is a regression test: Xen Orchestra sometimes
// returns a plain string as the task "result" (including on nested
// sub-tasks). The raw REST path must pass it through untouched.
func TestTaskGetStringResult(t *testing.T) {
	server := fakeXOGet(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTaskStringResult)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "888", "--output", "json")
	if err != nil {
		t.Fatalf("task get --output json (string result): %v", err)
	}

	var task map[string]any
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if task["result"] != "a plain string result" {
		t.Fatalf("expected string result to pass through, got %v", task["result"])
	}
	nested, ok := task["tasks"].([]any)
	if !ok || len(nested) != 1 {
		t.Fatalf("expected one nested task, got %v", task["tasks"])
	}
	child, _ := nested[0].(map[string]any)
	if child["result"] != "nested string result" {
		t.Fatalf("expected nested string result, got %v", child["result"])
	}
}

func TestTaskGetQuery(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "f6e5d4c3b2a1", "--query", "result.message")
	if err != nil {
		t.Fatalf("task get --query: %v", err)
	}
	if strings.TrimSpace(out) != "VM not found" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestTaskGetNotFound(t *testing.T) {
	server := fakeXOGet(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"not found"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runGet(t, "get", "f6e5d4c3b2a1")
	if err == nil {
		t.Fatal("expected an error when the task does not exist")
	}
	if !strings.Contains(err.Error(), "task \"f6e5d4c3b2a1\" not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskGetAPIError(t *testing.T) {
	server := fakeXOGet(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"boom"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runGet(t, "get", "f6e5d4c3b2a1")
	if err == nil {
		t.Fatal("expected an error when the API fails")
	}
	if !strings.Contains(err.Error(), "cannot get task") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskGetBadID(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runGet(t, "get", "a/b"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
}

// A successful task whose result is a plain string shows it as "Result" (no
// error block), and the duration is computed from the numeric start.
func TestTaskGetDetailStringResult(t *testing.T) {
	server := fakeXOGet(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTaskStringResult)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "888")
	if err != nil {
		t.Fatalf("task get: %v", err)
	}
	for _, expected := range []string{
		"Task 888  (success)",
		output.DetailField("Result", "a plain string result"),
		output.DetailField("Subtasks", "1"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
	if strings.Contains(out, "Error") {
		t.Errorf("a successful task must not show an error block:\n%s", out)
	}
}
