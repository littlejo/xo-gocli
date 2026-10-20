package vm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/testutil"
)

const fixtureVDIs = `[
	{
		"id": "11045407-4764-4c1c-8865-63f89d686b1b",
		"uuid": "11045407-4764-4c1c-8865-63f89d686b1b",
		"type": "VDI",
		"name_label": "web-01-disk-0",
		"size": 21474836480,
		"usage": 10737418240,
		"VDI_type": "user",
		"missing": false,
		"$SR": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	},
	{
		"id": "22045407-4764-4c1c-8865-63f89d686b1c",
		"uuid": "22045407-4764-4c1c-8865-63f89d686b1c",
		"type": "VDI",
		"name_label": "web-01-disk-1",
		"size": 10737418240,
		"usage": 5368709120,
		"VDI_type": "user",
		"missing": false,
		"$SR": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	}
]`

// vdiServer is a fake XO server for the per-VM VDI listing. It records every
// request so tests can assert on the path and query, and answers:
//
//	GET /rest/v0/vms/{id}         -> fixtureVM
//	GET /rest/v0/vms/{id}/vdis    -> fixtureVDIs
type vdiServer struct {
	*httptest.Server
	requests []request
}

func newVDIServer(t *testing.T) *vdiServer {
	t.Helper()
	s := &vdiServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := readBody(r)
		s.requests = append(s.requests, request{Method: r.Method, Path: r.URL.Path, Body: body, Query: r.URL.RawQuery})

		if !testutil.Authorized(w, r) {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/vdis"):
			_, _ = fmt.Fprint(w, fixtureVDIs)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			_, _ = fmt.Fprint(w, fixtureVM)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

// vdisRequest returns the GET request for the /vdis sub-resource, if any.
func (s *vdiServer) vdisRequest() (request, bool) {
	for _, r := range s.requests {
		if r.Method == http.MethodGet && strings.HasSuffix(r.Path, "/vdis") {
			return r, true
		}
	}
	return request{}, false
}

func TestVMVdisTable(t *testing.T) {
	server := newVDIServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "vdis", "550e8400-e29b-41d4-a716-446655440001")
	if err != nil {
		t.Fatalf("vm vdis: %v", err)
	}
	for _, expected := range []string{"ID", "NAME", "TYPE", "SIZE", "USAGE", "SR", "web-01-disk-0", "web-01-disk-1", "user", "21.47GB", "10.74GB"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
	if strings.Count(out, "web-01-disk-0") != 1 || strings.Count(out, "web-01-disk-1") != 1 {
		t.Fatalf("unexpected number of rows:\n%s", out)
	}

	req, ok := server.vdisRequest()
	if !ok {
		t.Fatal("expected a GET /vdis request")
	}
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/vdis" {
		t.Fatalf("unexpected vdis path: %s", req.Path)
	}
}

func TestVMVdisJSON(t *testing.T) {
	server := newVDIServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "vdis", "550e8400-e29b-41d4-a716-446655440001", "--output", "json")
	if err != nil {
		t.Fatalf("vm vdis --output json: %v", err)
	}
	var vdis []map[string]any
	if err := json.Unmarshal([]byte(out), &vdis); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(vdis) != 2 {
		t.Fatalf("expected 2 VDIs, got %d:\n%s", len(vdis), out)
	}
	if vdis[0]["name_label"] != "web-01-disk-0" {
		t.Fatalf("unexpected VDI payload: %s", out)
	}
}

func TestVMVdisQuery(t *testing.T) {
	server := newVDIServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "vdis", "550e8400-e29b-41d4-a716-446655440001", "--output", "json", "--query", "[].name_label")
	if err != nil {
		t.Fatalf("vm vdis --query: %v", err)
	}
	var names []string
	if err := json.Unmarshal([]byte(out), &names); err != nil {
		t.Fatalf("query output is not valid JSON: %v\n%s", err, out)
	}
	if len(names) != 2 || names[0] != "web-01-disk-0" || names[1] != "web-01-disk-1" {
		t.Fatalf("unexpected query result: %s", out)
	}
}

func TestVMVdisTypeFilter(t *testing.T) {
	server := newVDIServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "vdis", "550e8400-e29b-41d4-a716-446655440001", "--type", "user"); err != nil {
		t.Fatalf("vm vdis --type user: %v", err)
	}
	req, _ := server.vdisRequest()
	values, err := url.ParseQuery(req.Query)
	if err != nil {
		t.Fatalf("cannot parse query %q: %v", req.Query, err)
	}
	if values.Get("filter") != "VDI_type:user" {
		t.Fatalf("expected filter=VDI_type:user, got %q", values.Get("filter"))
	}
}

func TestVMVdisLimit(t *testing.T) {
	server := newVDIServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "vdis", "550e8400-e29b-41d4-a716-446655440001", "--limit", "5"); err != nil {
		t.Fatalf("vm vdis --limit 5: %v", err)
	}
	req, _ := server.vdisRequest()
	if !strings.Contains(req.Query, "limit=5") {
		t.Fatalf("expected limit=5 in the query, got %q", req.Query)
	}
}

func TestVMVdisBadID(t *testing.T) {
	server := newVDIServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "vdis", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
	if _, ok := server.vdisRequest(); ok {
		t.Fatal("no /vdis request must be sent for an invalid id")
	}
}

func TestVMVdisNotFound(t *testing.T) {
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

	_, err := runVM(t, "vm", "vdis", "550e8400-e29b-41d4-a716-446655440001")
	if err == nil {
		t.Fatal("expected an error when the VM does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
