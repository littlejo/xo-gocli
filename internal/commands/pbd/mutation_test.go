package pbd

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

const (
	getPBDID   = "66666666-6666-4666-8666-666666666666"
	pbdTask    = "77777777-7777-4777-8777-777777777777"
	pbdHostID  = "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	pbdSRID    = "aaaaaaaa-bbbb-cccc-dddd-000000000002"
	pbdPoolID  = "aaaaaaaa-bbbb-cccc-dddd-000000000009"
	otherPBDID = "66666666-6666-4666-8666-666666666667"
)

const fixturePBD = `{
	"id": "66666666-6666-4666-8666-666666666666",
	"uuid": "66666666-6666-4666-8666-666666666666",
	"type": "PBD",
	"$pool": "aaaaaaaa-bbbb-cccc-dddd-000000000009",
	"_xapiRef": "PBD:111",
	"attached": true,
	"host": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
	"SR": "aaaaaaaa-bbbb-cccc-dddd-000000000002",
	"device_config": {"device": "/dev/sda"},
	"otherConfig": {}
}`

const fixturePBDs = `[
	` + fixturePBD + `,
	{
		"id": "66666666-6666-4666-8666-666666666667",
		"uuid": "66666666-6666-4666-8666-666666666667",
		"type": "PBD",
		"$pool": "aaaaaaaa-bbbb-cccc-dddd-000000000009",
		"attached": false,
		"host": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
		"SR": "aaaaaaaa-bbbb-cccc-dddd-000000000003",
		"device_config": {"server": "nfs-host", "serverpath": "/export"},
		"otherConfig": {}
	}
]`

type request struct {
	Method string
	Path   string
	Body   string
}

// mutationServer is a fake XO server for the PBD endpoints. It records every
// request so tests can assert on method, path and body, and answers:
//
//	GET    /rest/v0/pbds                          -> fixturePBDs
//	GET    /rest/v0/pbds/{id}                     -> fixturePBD (404 for other ids)
//	POST   /rest/v0/pbds/{id}/actions/plug        -> {"taskId": pbdTask}
//	POST   /rest/v0/pbds/{id}/actions/unplug      -> {"taskId": pbdTask}
//	GET    /rest/v0/tasks/{id}                    -> a success task
type mutationServer struct {
	*httptest.Server
	requests []request
}

func newMutationServer(t *testing.T) *mutationServer {
	t.Helper()
	s := &mutationServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := readBody(r)
		s.requests = append(s.requests, request{Method: r.Method, Path: r.URL.Path, Body: body})

		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/pbds":
			_, _ = fmt.Fprint(w, fixturePBDs)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/pbds/"+getPBDID:
			_, _ = fmt.Fprint(w, fixturePBD)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/pbds/"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/tasks/"):
			_, _ = fmt.Fprint(w, `{"id":"`+pbdTask+`","status":"success","start":1700000000000,"end":1700000001000}`)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/v0/pbds/"+getPBDID+"/actions/plug":
			_, _ = fmt.Fprint(w, `{"taskId":"`+pbdTask+`"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/v0/pbds/"+getPBDID+"/actions/unplug":
			_, _ = fmt.Fprint(w, `{"taskId":"`+pbdTask+`"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func (s *mutationServer) requestByPathPrefix(prefix string) (request, bool) {
	for _, r := range s.requests {
		if strings.Contains(r.Path, prefix) {
			return r, true
		}
	}
	return request{}, false
}

func readBody(r *http.Request) (string, error) {
	data, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	return string(data), err
}

// isolatePointers points the CLI at the fake server and at a throwaway config
// file, and clears any ambient XOA_ variables.
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
	root.AddCommand(NewCommand())
	return root
}

func runPBD(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

// --- list --------------------------------------------------------------------

func TestPBDListTable(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runPBD(t, "pbd", "list")
	if err != nil {
		t.Fatalf("pbd list: %v", err)
	}
	for _, expected := range []string{"ID", "HOST", "SR", "POOL", "ATTACHED", "DEVICE", "/dev/sda"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
	if !strings.Contains(out, "yes") || !strings.Contains(out, "no") {
		t.Fatalf("table should show attached yes/no:\n%s", out)
	}
}

func TestPBDListJSON(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runPBD(t, "pbd", "list", "--output", "json")
	if err != nil {
		t.Fatalf("pbd list --output json: %v", err)
	}
	var pbds []map[string]any
	if err := json.Unmarshal([]byte(out), &pbds); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(pbds) != 2 {
		t.Fatalf("expected 2 PBDs, got %d: %s", len(pbds), out)
	}
}

// --- get ---------------------------------------------------------------------

// TestPBDGetDetail checks the human view of `pbd get`: it is a detail sheet,
// not the one-row table shared with `pbd list`. The mutation server does not
// serve hosts/SRs/pools, so the relationships fall back to their raw ids; the
// full device_config is shown (here: device=/dev/sda).
func TestPBDGetDetail(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runPBD(t, "pbd", "get", getPBDID)
	if err != nil {
		t.Fatalf("pbd get: %v", err)
	}
	for _, expected := range []string{
		"PBD /dev/sda  (attached=yes)",
		output.DetailField("Host", pbdHostID),
		output.DetailField("SR", pbdSRID),
		output.DetailField("Pool", pbdPoolID),
		output.DetailField("Config", "device=/dev/sda"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

func TestPBDGetJSON(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runPBD(t, "pbd", "get", getPBDID, "--output", "json")
	if err != nil {
		t.Fatalf("pbd get --output json: %v", err)
	}
	var pbd map[string]any
	if err := json.Unmarshal([]byte(out), &pbd); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if !pbd["attached"].(bool) {
		t.Fatalf("expected attached=true, got: %s", out)
	}
}

func TestPBDGetNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	_, err := runPBD(t, "pbd", "get", missing)
	if err == nil {
		t.Fatal("expected an error when the PBD does not exist")
	}
	if !strings.Contains(err.Error(), `PBD "99999999-9999-4999-8999-999999999999" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPBDGetBadID(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runPBD(t, "pbd", "get", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
}

// --- plug / unplug -----------------------------------------------------------

func TestPBDPlug(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runPBD(t, "pbd", "plug", getPBDID)
	if err != nil {
		t.Fatalf("pbd plug: %v", err)
	}
	if !strings.Contains(out, "plug") || !strings.Contains(out, pbdTask) {
		t.Fatalf("unexpected plug output: %q", out)
	}
	req, ok := server.requestByPathPrefix("/actions/plug")
	if !ok {
		t.Fatal("expected a POST plug request")
	}
	if req.Path != "/rest/v0/pbds/"+getPBDID+"/actions/plug" {
		t.Fatalf("unexpected plug path: %s", req.Path)
	}
}

func TestPBDUnplug(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runPBD(t, "pbd", "unplug", getPBDID)
	if err != nil {
		t.Fatalf("pbd unplug: %v", err)
	}
	if !strings.Contains(out, "unplug") || !strings.Contains(out, pbdTask) {
		t.Fatalf("unexpected unplug output: %q", out)
	}
	req, ok := server.requestByPathPrefix("/actions/unplug")
	if !ok {
		t.Fatal("expected a POST unplug request")
	}
	if req.Path != "/rest/v0/pbds/"+getPBDID+"/actions/unplug" {
		t.Fatalf("unexpected unplug path: %s", req.Path)
	}
}

func TestPBDPlugJSON(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runPBD(t, "pbd", "plug", getPBDID, "--output", "json")
	if err != nil {
		t.Fatalf("pbd plug --output json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("plug JSON output is not valid JSON: %v\n%s", err, out)
	}
	if doc["task_id"] != pbdTask || doc["action"] != "plug" {
		t.Fatalf("unexpected plug JSON payload: %s", out)
	}
}

func TestPBDPlugNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	_, err := runPBD(t, "pbd", "plug", missing)
	if err == nil {
		t.Fatal("expected an error when the PBD does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := server.requestByPathPrefix("/actions/plug"); ok {
		t.Fatal("plug must not be sent for a missing PBD")
	}
}

// --- --wait ------------------------------------------------------------------

func TestPBDPlugWait(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runPBD(t, "pbd", "plug", getPBDID, "--wait")
	if err != nil {
		t.Fatalf("pbd plug --wait: %v", err)
	}
	// The completed task is rendered instead of the "Requested plug" line.
	for _, expected := range []string{"ID", "STATUS", pbdTask, "success"} {
		if !strings.Contains(out, expected) {
			t.Errorf("wait output missing %q:\n%s", expected, out)
		}
	}
	if strings.Contains(out, "Requested plug") {
		t.Fatalf("the plain action line must be replaced by the task when --wait is set:\n%s", out)
	}
}
