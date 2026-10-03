package vbd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	getVBDID     = "33333333-3333-4333-8333-333333333333"
	createdVBDID = "55555555-5555-4555-8555-555555555555"
	vmID         = "550e8400-e29b-41d4-a716-446655440001"
	vdiID        = "22222222-2222-4222-8222-222222222222"
)

const fixtureVBD = `{
	"id": "33333333-3333-4333-8333-333333333333",
	"uuid": "33333333-3333-4333-8333-333333333333",
	"type": "VBD",
	"$pool": "aaaaaaaa-bbbb-cccc-dddd-000000000009",
	"_xapiRef": "VBD:111",
	"attached": true,
	"bootable": false,
	"device": "xvda",
	"is_cd_drive": false,
	"position": "0",
	"read_only": false,
	"VDI": "11111111-1111-4111-8111-111111111111",
	"VM": "550e8400-e29b-41d4-a716-446655440001"
}`

type request struct {
	Method string
	Path   string
	Body   string
}

// mutationServer is a fake XO server for the VBD endpoints. It records every
// request so tests can assert on method, path and body, and answers:
//
//	GET    /rest/v0/vbds/{id}                     -> fixtureVBD (404 for other ids)
//	POST   /rest/v0/vbds                          -> {"id": createdVBDID}
//	DELETE /rest/v0/vbds/{id}                     -> 200 {}
//	POST   /rest/v0/vbds/{id}/actions/connect     -> {"taskId":"task-123"}
//	POST   /rest/v0/vbds/{id}/actions/disconnect  -> {"taskId":"task-123"}
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
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/vbds/"+getVBDID:
			_, _ = fmt.Fprint(w, fixtureVBD)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/tasks/"):
			_, _ = fmt.Fprint(w, `{"id":"task-123","status":"success","start":1700000000000,"end":1700000001000}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/vbds/"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/v0/vbds":
			_, _ = fmt.Fprint(w, `{"id": "55555555-5555-4555-8555-555555555555"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/v0/vbds/"+getVBDID:
			_, _ = fmt.Fprint(w, `{}`)
		case (r.Method == http.MethodPost) && strings.Contains(r.URL.Path, "/actions/connect"):
			_, _ = fmt.Fprint(w, `{"taskId":"task-123"}`)
		case (r.Method == http.MethodPost) && strings.Contains(r.URL.Path, "/actions/disconnect"):
			_, _ = fmt.Fprint(w, `{"taskId":"task-123"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func (s *mutationServer) requestByMethod(method string) (request, bool) {
	for _, r := range s.requests {
		if r.Method == method {
			return r, true
		}
	}
	return request{}, false
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

// --- get --------------------------------------------------------------------

// TestVBDGetDetail checks the human view of `vbd get`: it is a detail sheet,
// not the one-row table shared with `vbd list`. The mutation server does not
// serve VMs or VDIs, so the relationships fall back to their raw ids.
func TestVBDGetDetail(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "get", getVBDID)
	if err != nil {
		t.Fatalf("vbd get: %v", err)
	}
	for _, expected := range []string{
		"VBD xvda  (RW, attached=yes)",
		output.DetailField("Device", "xvda"),
		output.DetailField("VM", vmID),
		output.DetailField("VDI", "11111111-1111-4111-8111-111111111111"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

func TestVBDGetJSON(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "get", getVBDID, "--output", "json")
	if err != nil {
		t.Fatalf("vbd get --output json: %v", err)
	}
	var vbd map[string]any
	if err := json.Unmarshal([]byte(out), &vbd); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if vbd["device"] != "xvda" {
		t.Fatalf("unexpected VBD payload: %s", out)
	}
}

func TestVBDGetNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	_, err := runVBD(t, "vbd", "get", missing)
	if err == nil {
		t.Fatal("expected an error when the VBD does not exist")
	}
	if !strings.Contains(err.Error(), `VBD "99999999-9999-4999-8999-999999999999" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- create -----------------------------------------------------------------

func TestVBDCreate(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "create", "--vm", vmID, "--vdi", vdiID)
	if err != nil {
		t.Fatalf("vbd create: %v", err)
	}
	if !strings.Contains(out, "created") || !strings.Contains(out, createdVBDID) {
		t.Fatalf("create output should report the created VBD id:\n%s", out)
	}
	// The next-step hint should point at the hot-plug command.
	if !strings.Contains(out, "xo vbd connect") {
		t.Fatalf("create output should hint the connect command:\n%s", out)
	}

	req, ok := server.requestByMethod(http.MethodPost)
	if !ok {
		t.Fatal("expected a POST create request")
	}
	if req.Path != "/rest/v0/vbds" {
		t.Fatalf("unexpected create path: %s", req.Path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse create body: %v\n%s", err, req.Body)
	}
	if body["VM"] != vmID {
		t.Fatalf("expected VM, got %s", req.Body)
	}
	if body["VDI"] != vdiID {
		t.Fatalf("expected VDI, got %s", req.Body)
	}
	if body["type"] != "Disk" {
		t.Fatalf("expected type=Disk, got %s", req.Body)
	}
	if _, present := body["mode"]; present {
		t.Fatalf("mode must be omitted when --mode is not given: %s", req.Body)
	}
	if _, present := body["bootable"]; present {
		t.Fatalf("bootable must be omitted when --bootable is not given: %s", req.Body)
	}
}

func TestVBDCreateWithModeAndBootable(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runVBD(t, "vbd", "create", "--vm", vmID, "--vdi", vdiID, "--mode", "RO", "--bootable"); err != nil {
		t.Fatalf("vbd create --mode RO --bootable: %v", err)
	}
	req, _ := server.requestByMethod(http.MethodPost)
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse create body: %v\n%s", err, req.Body)
	}
	if body["mode"] != "RO" {
		t.Fatalf("expected mode=RO, got %s", req.Body)
	}
	if got, _ := body["bootable"].(bool); !got {
		t.Fatalf("expected bootable=true, got %s", req.Body)
	}
}

func TestVBDCreateRequiresVMAndVDI(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runVBD(t, "vbd", "create", "--vdi", vdiID); err == nil {
		t.Fatal("expected an error when --vm is missing")
	}
	if _, err := runVBD(t, "vbd", "create", "--vm", vmID); err == nil {
		t.Fatal("expected an error when --vdi is missing")
	}
	if _, err := runVBD(t, "vbd", "create", "--vm", vmID, "--vdi", vdiID, "--mode", "bogus"); err == nil {
		t.Fatal("expected an error for an invalid --mode")
	}
	// The first POST match is the create endpoint only when a create was
	// attempted; none of the three calls above may have POSTed to /vbds.
	if req, ok := server.requestByMethod(http.MethodPost); ok && req.Path == "/rest/v0/vbds" {
		t.Fatal("create must not be executed with missing/invalid flags")
	}
}

// --- delete -----------------------------------------------------------------

func TestVBDDelete(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "delete", getVBDID, "--yes")
	if err != nil {
		t.Fatalf("vbd delete: %v", err)
	}
	// The label links the VM and VDI ids.
	if !strings.Contains(out, vmID) || !strings.Contains(out, "11111111-1111-4111-8111-111111111111") {
		t.Fatalf("delete output should mention the VM and VDI:\n%s", out)
	}
	if !strings.Contains(out, "xo vdi delete") {
		t.Fatalf("delete output should hint the VDI delete:\n%s", out)
	}
	if _, ok := server.requestByMethod(http.MethodDelete); !ok {
		t.Fatal("expected a DELETE request")
	}
}

func TestVBDDeleteRequiresConfirmation(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runVBD(t, "vbd", "delete", getVBDID)
	if err == nil {
		t.Fatal("expected an error when no confirmation is given and stdin is not a terminal")
	}
	if _, ok := server.requestByMethod(http.MethodDelete); ok {
		t.Fatal("the VBD must not be deleted without confirmation")
	}
}

func TestVBDDeleteNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	_, err := runVBD(t, "vbd", "delete", missing, "--yes")
	if err == nil {
		t.Fatal("expected an error when the VBD does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := server.requestByMethod(http.MethodDelete); ok {
		t.Fatal("the VBD must not be deleted when it does not exist")
	}
}

// --- connect / disconnect ----------------------------------------------------

func TestVBDConnect(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "connect", getVBDID)
	if err != nil {
		t.Fatalf("vbd connect: %v", err)
	}
	if !strings.Contains(out, "connect") || !strings.Contains(out, "task-123") {
		t.Fatalf("unexpected connect output: %q", out)
	}
	req, ok := server.requestByPathPrefix("/actions/connect")
	if !ok {
		t.Fatal("expected a POST connect request")
	}
	if req.Path != "/rest/v0/vbds/"+getVBDID+"/actions/connect" {
		t.Fatalf("unexpected connect path: %s", req.Path)
	}
}

func TestVBDDisconnect(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "disconnect", getVBDID)
	if err != nil {
		t.Fatalf("vbd disconnect: %v", err)
	}
	if !strings.Contains(out, "disconnect") || !strings.Contains(out, "task-123") {
		t.Fatalf("unexpected disconnect output: %q", out)
	}
	req, ok := server.requestByPathPrefix("/actions/disconnect")
	if !ok {
		t.Fatal("expected a POST disconnect request")
	}
	if req.Path != "/rest/v0/vbds/"+getVBDID+"/actions/disconnect" {
		t.Fatalf("unexpected disconnect path: %s", req.Path)
	}
}

func TestVBDConnectNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	_, err := runVBD(t, "vbd", "connect", missing)
	if err == nil {
		t.Fatal("expected an error when the VBD does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := server.requestByPathPrefix("/actions/connect"); ok {
		t.Fatal("connect must not be sent for a missing VBD")
	}
}

// --- --wait ------------------------------------------------------------------

func TestVBDConnectWait(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "connect", getVBDID, "--wait")
	if err != nil {
		t.Fatalf("vbd connect --wait: %v", err)
	}
	// The completed task is rendered instead of the "Requested connect" line.
	for _, expected := range []string{"ID", "STATUS", "task-123", "success"} {
		if !strings.Contains(out, expected) {
			t.Errorf("wait output missing %q:\n%s", expected, out)
		}
	}
	if strings.Contains(out, "Requested connect") {
		t.Fatalf("the plain action line must be replaced by the task when --wait is set:\n%s", out)
	}
}
