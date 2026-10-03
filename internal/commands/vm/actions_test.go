package vm

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
	"github.com/littlejo/xo-gocli/internal/output"
)

const fixtureVM = `{
	"id": "550e8400-e29b-41d4-a716-446655440001",
	"uuid": "550e8400-e29b-41d4-a716-446655440001",
	"type": "vm",
	"name_label": "web-01",
	"power_state": "Halted",
	"memory": {"size": 2147483648},
	"CPUs": {"number": 2},
	"$container": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
}`

const fixtureTask = `{
	"id": "task-123",
	"status": "success",
	"start": 1700000000000,
	"end": 1700000001000
}`

// actionServer is a fake XO server for the per-VM endpoints. It records every
// request so tests can assert on the method, path and body, and answers:
//
//	GET  /rest/v0/vms/{id}            -> fixtureVM
//	GET  /rest/v0/tasks/{id}          -> fixtureTask
//	POST /rest/v0/vms/{id}/actions/*  -> {"taskId":"task-123"}
type actionServer struct {
	*httptest.Server
	t        *testing.T
	requests []request
}

type request struct {
	Method string
	Path   string
	Body   string
	Query  string
}

func newActionServer(t *testing.T) *actionServer {
	t.Helper()
	s := &actionServer{t: t}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := readBody(r)
		s.requests = append(s.requests, request{Method: r.Method, Path: r.URL.Path, Body: body, Query: r.URL.RawQuery})

		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			_, _ = fmt.Fprint(w, fixtureVM)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/tasks/"):
			_, _ = fmt.Fprint(w, fixtureTask)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/actions/"):
			_, _ = fmt.Fprint(w, `{"taskId":"task-123"}`)
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

func readBody(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	data, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	return string(data), err
}

// isolateVM points the CLI at the fake server and a throwaway config file.
func isolateVM(t *testing.T, url string) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE", "XOA_YES", "XOA_WAIT"} {
		t.Setenv(key, "")
	}
	t.Setenv("XOA_ENDPOINT", url)
	t.Setenv("XOA_TOKEN", "test-token")
}

func newVMTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.AddCommand(NewCommand())
	return root
}

func runVM(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newVMTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

// --- get --------------------------------------------------------------------

// TestVMGetDetail checks the human (default) view of `vm get`: it is now a
// detail sheet, not the one-row table shared with `vm list`. The action server
// does not serve hosts or pools, so the container falls back to its raw id.
func TestVMGetDetail(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "get", "550e8400-e29b-41d4-a716-446655440001")
	if err != nil {
		t.Fatalf("vm get: %v", err)
	}
	for _, expected := range []string{
		"VM web-01  (Halted)",
		output.DetailField("Memory", "2.147GB"),
		output.DetailField("CPUs", "2"),
		output.DetailField("Container", "aaaaaaaa-bbbb-cccc-dddd-000000000001"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
	// The old one-row table columns must be gone from the detail sheet.
	for _, gone := range []string{"NAME", "POWER STATE", "HOST/POOL"} {
		if strings.Contains(out, gone) {
			t.Errorf("get output must not contain the old table column %q:\n%s", gone, out)
		}
	}
}

func TestVMGetJSON(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "get", "550e8400-e29b-41d4-a716-446655440001", "--output", "json")
	if err != nil {
		t.Fatalf("vm get --output json: %v", err)
	}
	var vm map[string]any
	if err := json.Unmarshal([]byte(out), &vm); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if vm["name_label"] != "web-01" {
		t.Fatalf("unexpected VM payload: %s", out)
	}
}

func TestVMGetNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rest/v0/vms/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	isolateVM(t, server.URL)

	_, err := runVM(t, "vm", "get", "550e8400-e29b-41d4-a716-446655440001")
	if err == nil {
		t.Fatal("expected an error when the VM does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVMGetBadID(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "get", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
}

// --- start ------------------------------------------------------------------

func TestVMStart(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "start", "550e8400-e29b-41d4-a716-446655440001")
	if err != nil {
		t.Fatalf("vm start: %v", err)
	}
	if !strings.Contains(out, "start") || !strings.Contains(out, "web-01") || !strings.Contains(out, "task-123") {
		t.Fatalf("unexpected start output: %q", out)
	}
	req, ok := server.post()
	if !ok {
		t.Fatal("expected a POST action request")
	}
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/start" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

func TestVMStartWithHost(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	host := "6b7c8d9e-0000-1111-2222-333344445555"
	if _, err := runVM(t, "vm", "start", "550e8400-e29b-41d4-a716-446655440001", "--host", host); err != nil {
		t.Fatalf("vm start --host: %v", err)
	}
	req, _ := server.post()
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse request body: %v\n%s", err, req.Body)
	}
	if body["hostId"] != host {
		t.Fatalf("expected hostId=%s in body, got %s", host, req.Body)
	}
}

// --- stop -------------------------------------------------------------------

func TestVMStopClean(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "stop", "550e8400-e29b-41d4-a716-446655440001", "--yes"); err != nil {
		t.Fatalf("vm stop: %v", err)
	}
	req, _ := server.post()
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/clean_shutdown" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

func TestVMStopHard(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "stop", "550e8400-e29b-41d4-a716-446655440001", "--hard", "--yes"); err != nil {
		t.Fatalf("vm stop --hard: %v", err)
	}
	req, _ := server.post()
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/hard_shutdown" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

func TestVMStopRequiresConfirmation(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	_, err := runVM(t, "vm", "stop", "550e8400-e29b-41d4-a716-446655440001")
	if err == nil {
		t.Fatal("expected an error when no confirmation is given and stdin is not a terminal")
	}
	if _, ok := server.post(); ok {
		t.Fatal("the action must not be executed without confirmation")
	}
}

// $XOA_YES is the script counterpart of --yes: it skips the confirmation
// without a terminal.

func TestVMStopSkipsConfirmationWithEnvYes(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	t.Setenv("XOA_YES", "1")

	if _, err := runVM(t, "vm", "stop", "550e8400-e29b-41d4-a716-446655440001"); err != nil {
		t.Fatalf("vm stop with XOA_YES=1: %v", err)
	}
	req, _ := server.post()
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/clean_shutdown" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

// --- reboot -----------------------------------------------------------------

func TestVMRebootClean(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "reboot", "550e8400-e29b-41d4-a716-446655440001"); err != nil {
		t.Fatalf("vm reboot: %v", err)
	}
	req, _ := server.post()
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/clean_reboot" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

func TestVMRebootHard(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "reboot", "550e8400-e29b-41d4-a716-446655440001", "--hard"); err != nil {
		t.Fatalf("vm reboot --hard: %v", err)
	}
	req, _ := server.post()
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/hard_reboot" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

// --- pause / unpause / suspend / resume -------------------------------------

func TestVMPause(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "pause", "550e8400-e29b-41d4-a716-446655440001")
	if err != nil {
		t.Fatalf("vm pause: %v", err)
	}
	if !strings.Contains(out, "pause") || !strings.Contains(out, "web-01") || !strings.Contains(out, "task-123") {
		t.Fatalf("unexpected pause output: %q", out)
	}
	req, ok := server.post()
	if !ok {
		t.Fatal("expected a POST action request")
	}
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/pause" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

func TestVMUnpause(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "unpause", "550e8400-e29b-41d4-a716-446655440001"); err != nil {
		t.Fatalf("vm unpause: %v", err)
	}
	req, _ := server.post()
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/unpause" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

func TestVMSuspend(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "suspend", "550e8400-e29b-41d4-a716-446655440001"); err != nil {
		t.Fatalf("vm suspend: %v", err)
	}
	req, _ := server.post()
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/suspend" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

func TestVMResume(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "resume", "550e8400-e29b-41d4-a716-446655440001"); err != nil {
		t.Fatalf("vm resume: %v", err)
	}
	req, _ := server.post()
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/resume" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
}

// These four are reversible, so unlike stop/delete they must run without any
// confirmation even when stdin is not a terminal.

func TestVMPauseRequiresNoConfirmation(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "pause", "550e8400-e29b-41d4-a716-446655440001"); err != nil {
		t.Fatalf("pause must not require confirmation: %v", err)
	}
}

func TestVMSuspendRequiresNoConfirmation(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "suspend", "550e8400-e29b-41d4-a716-446655440001"); err != nil {
		t.Fatalf("suspend must not require confirmation: %v", err)
	}
}

// --- snapshot ---------------------------------------------------------------

func TestVMSnapshot(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "snapshot", "550e8400-e29b-41d4-a716-446655440001", "--name", "before-upgrade"); err != nil {
		t.Fatalf("vm snapshot: %v", err)
	}
	req, _ := server.post()
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/actions/snapshot" {
		t.Fatalf("unexpected action path: %s", req.Path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse request body: %v\n%s", err, req.Body)
	}
	if body["name_label"] != "before-upgrade" {
		t.Fatalf("expected name_label=before-upgrade, got %s", req.Body)
	}
}

// $XOA_WAIT is the script counterpart of --wait: it makes the action wait for
// its task without passing the flag on every command.
func TestVMStartWaitsWithEnvWait(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	t.Setenv("XOA_WAIT", "1")

	out, err := runVM(t, "vm", "start", "550e8400-e29b-41d4-a716-446655440001")
	if err != nil {
		t.Fatalf("vm start with XOA_WAIT=1: %v", err)
	}
	if !strings.Contains(out, "STATUS") || !strings.Contains(out, "task-123") {
		t.Fatalf("XOA_WAIT=1 should wait and render the task:\n%s", out)
	}
	if strings.Contains(out, "Requested start") {
		t.Fatalf("the plain action line must be replaced by the task:\n%s", out)
	}
}

// A value that is not "true-ish" must not enable the wait.
func TestVMStartEnvWaitInvalidValue(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	t.Setenv("XOA_WAIT", "nope")

	out, err := runVM(t, "vm", "start", "550e8400-e29b-41d4-a716-446655440001")
	if err != nil {
		t.Fatalf("vm start: %v", err)
	}
	if !strings.Contains(out, "Requested start") {
		t.Fatalf("a non-true XOA_WAIT must not wait:\n%s", out)
	}
}

// --- shared error handling --------------------------------------------------

func TestVMActionAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rest/v0/vms/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, fixtureVM)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"boom"}`)
	}))
	defer server.Close()
	isolateVM(t, server.URL)

	_, err := runVM(t, "vm", "start", "550e8400-e29b-41d4-a716-446655440001")
	if err == nil {
		t.Fatal("expected an error when the API fails")
	}
	if !strings.Contains(err.Error(), "cannot start") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- --wait -----------------------------------------------------------------

func TestVMStartWait(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "start", "550e8400-e29b-41d4-a716-446655440001", "--wait")
	if err != nil {
		t.Fatalf("vm start --wait: %v", err)
	}
	// The completed task is rendered instead of the "Requested start" line.
	for _, expected := range []string{"ID", "STATUS", "task-123", "success"} {
		if !strings.Contains(out, expected) {
			t.Errorf("wait output missing %q:\n%s", expected, out)
		}
	}
	if strings.Contains(out, "Requested start") {
		t.Fatalf("the plain action line must be replaced by the task when --wait is set:\n%s", out)
	}
}

func TestVMStartWaitJSON(t *testing.T) {
	server := newActionServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "start", "550e8400-e29b-41d4-a716-446655440001", "--wait", "--output", "json")
	if err != nil {
		t.Fatalf("vm start --wait --output json: %v", err)
	}
	var task map[string]any
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("wait JSON output is not valid JSON: %v\n%s", err, out)
	}
	if task["id"] != "task-123" || task["status"] != "success" {
		t.Fatalf("unexpected task payload: %s", out)
	}
}

// A server whose action succeeds but whose task ends in failure: --wait must
// still render the completed task and exit non-zero with the task message.
func TestVMStartWaitFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rest/v0/vms/"):
			_, _ = fmt.Fprint(w, fixtureVM)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rest/v0/tasks/"):
			_, _ = fmt.Fprint(w, `{"id":"task-123","status":"failure","start":1700000000000,"end":1700000001000,"result":{"message":"disk full"}}`)
		case r.Method == http.MethodPost:
			_, _ = fmt.Fprint(w, `{"taskId":"task-123"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "start", "550e8400-e29b-41d4-a716-446655440001", "--wait")
	if err == nil {
		t.Fatal("expected a non-zero exit when the waited task fails")
	}
	if !strings.Contains(err.Error(), "task task-123 failed: disk full") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "failure") {
		t.Fatalf("the completed task must still be rendered:\n%s", out)
	}
}
