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

const fixtureTasks = `[
	{
		"id": "a1b2c3d4e5f6",
		"status": "success",
		"start": "2026-09-28T10:00:00.000Z",
		"end": "2026-09-28T10:00:01.500Z",
		"properties": {
			"name": "start",
			"type": "VM",
			"objectId": "550e8400-e29b-41d4-a716-446655440001"
		}
	},
	{
		"id": "f6e5d4c3b2a1",
		"status": "failure",
		"start": "2026-09-28T10:05:00.000Z",
		"properties": {
			"name": "clean_shutdown",
			"type": "VM",
			"objectId": "550e8400-e29b-41d4-a716-446655440002"
		}
	},
	{
		"id": "0a1b2c3d4e5f",
		"status": "success",
		"start": 1700000000000,
		"end": "2026-09-28T11:00:00.000Z",
		"properties": {
			"name": "provisioning",
			"type": "Pool",
			"objectId": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
		},
		"result": "some string result",
		"tasks": [
			{
				"id": "998877665544",
				"status": "success",
				"start": "2026-09-28T11:00:00.000Z",
				"result": "child string result"
			}
		]
	}
]`

// fakeXO serves the minimal REST surface used by Task().GetAll.
func fakeXO(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/tasks" {
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
		_, _ = fmt.Fprint(w, fixtureTasks)
	}))
}

func isolatePointers(t *testing.T, url string) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE", "XOA_YES", "XOA_WAIT"} {
		t.Setenv(key, "")
	}
	t.Setenv("XOA_ENDPOINT", url)
	t.Setenv("XOA_TOKEN", "test-token")
}

func newTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	root.AddCommand(newListCommand())
	return root
}

func runList(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestTaskListTable(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("task list: %v", err)
	}
	for _, expected := range []string{"ID", "STATUS", "TYPE", "NAME", "STARTED", "ENDED", "a1b2c3d4e5f6", "f6e5d4c3b2a1", "0a1b2c3d4e5f", "success", "failure", "start", "clean_shutdown", "provisioning", "2026-09-28T10:00:00Z", "2023-11-14T22:13:20Z"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
}

func TestTaskListJSON(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("task list --output json: %v", err)
	}

	var tasks []map[string]any
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	if tasks[0]["id"] != "a1b2c3d4e5f6" || tasks[0]["status"] != "success" {
		t.Fatalf("unexpected task payload: %s", out)
	}
	// Regression: Xen Orchestra sometimes returns a plain string as the
	// task "result" (including on nested sub-tasks). The raw REST path must
	// pass it through untouched.
	if tasks[2]["result"] != "some string result" {
		t.Fatalf("expected string result to pass through, got %v", tasks[2]["result"])
	}
	nested, ok := tasks[2]["tasks"].([]any)
	if !ok || len(nested) != 1 {
		t.Fatalf("expected one nested task, got %v", tasks[2]["tasks"])
	}
	child, _ := nested[0].(map[string]any)
	if child["result"] != "child string result" {
		t.Fatalf("expected nested string result, got %v", child["result"])
	}
}

func TestTaskListQuery(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--query", "[?status==`failure`].id", "--output", "text")
	if err != nil {
		t.Fatalf("task list --query: %v", err)
	}
	if strings.TrimSpace(out) != "f6e5d4c3b2a1" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestTaskListQueryInvalid(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--query", "bad["); err == nil {
		t.Fatal("expected an error for an invalid query")
	}
}

func TestTaskListStatus(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter") != "status:failure" {
			t.Errorf("expected filter=status:failure, got %q", r.URL.Query().Get("filter"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTasks)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--status", "failure"); err != nil {
		t.Fatalf("task list --status: %v", err)
	}
}

func TestTaskListLimit(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "3" {
			t.Errorf("expected limit=3, got %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTasks)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--limit", "3"); err != nil {
		t.Fatalf("task list --limit: %v", err)
	}
}

func TestTaskListAPIError(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"boom"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runList(t, "list")
	if err == nil {
		t.Fatal("expected an error when the API fails")
	}
	if !strings.Contains(err.Error(), "cannot list tasks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskListNoCredentials(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE"} {
		t.Setenv(key, "")
	}

	_, err := runList(t, "list")
	if err == nil {
		t.Fatal("expected an error when nothing is configured")
	}
	if !strings.Contains(err.Error(), "no endpoint configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}
