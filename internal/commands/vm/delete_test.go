package vm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/testutil"
)

// deleteServer is a fake XO server for the VM delete endpoint. It answers:
//
//	GET    /rest/v0/vms/{id}  -> fixtureVM
//	DELETE /rest/v0/vms/{id}  -> 200 {"success":true}
//
// and records requests so tests can assert the delete actually went out (and
// that it did not when confirmation was refused).
type deleteServer struct {
	*httptest.Server
	requests []request
}

func newDeleteServer(t *testing.T) *deleteServer {
	t.Helper()
	s := &deleteServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests = append(s.requests, request{Method: r.Method, Path: r.URL.Path})

		if !testutil.Authorized(w, r) {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			_, _ = fmt.Fprint(w, fixtureVM)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			_, _ = fmt.Fprint(w, `{"success":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func (s *deleteServer) hasDelete() bool {
	for _, r := range s.requests {
		if r.Method == http.MethodDelete {
			return true
		}
	}
	return false
}

func (s *deleteServer) deletePath() string {
	for _, r := range s.requests {
		if r.Method == http.MethodDelete {
			return r.Path
		}
	}
	return ""
}

func TestVMDelete(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "delete", "550e8400-e29b-41d4-a716-446655440001", "--yes")
	if err != nil {
		t.Fatalf("vm delete: %v", err)
	}
	if !strings.Contains(out, "web-01") {
		t.Fatalf("delete output should mention the VM name:\n%s", out)
	}
	if !server.hasDelete() {
		t.Fatal("expected a DELETE request")
	}
	if server.deletePath() != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001" {
		t.Fatalf("unexpected delete path: %s", server.deletePath())
	}
}

func TestVMDeleteJSON(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "delete", "550e8400-e29b-41d4-a716-446655440001", "--yes", "--output", "json")
	if err != nil {
		t.Fatalf("vm delete --output json: %v", err)
	}
	// Machine output must be valid JSON carrying only the requested data.
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("delete JSON output is not valid JSON: %v\n%s", err, out)
	}
	if doc["vm"] != "web-01" {
		t.Fatalf("unexpected delete JSON payload: %s", out)
	}
}

func TestVMDeleteRequiresConfirmation(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	_, err := runVM(t, "vm", "delete", "550e8400-e29b-41d4-a716-446655440001")
	if err == nil {
		t.Fatal("expected an error when no confirmation is given and stdin is not a terminal")
	}
	if server.hasDelete() {
		t.Fatal("the VM must not be deleted without confirmation")
	}
}

func TestVMDeleteSkipsConfirmationWithEnvYes(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	isolateVM(t, server.URL)
	t.Setenv("XOA_YES", "1")

	if _, err := runVM(t, "vm", "delete", "550e8400-e29b-41d4-a716-446655440001"); err != nil {
		t.Fatalf("vm delete with XOA_YES=1: %v", err)
	}
	if !server.hasDelete() {
		t.Fatal("expected a DELETE request when XOA_YES=1")
	}
}

func TestVMDeleteBadID(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "delete", "not-a-uuid", "--yes"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
	if server.hasDelete() {
		t.Fatal("the VM must not be deleted for an invalid id")
	}
}

func TestVMDeleteNotFound(t *testing.T) {
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

	_, err := runVM(t, "vm", "delete", "550e8400-e29b-41d4-a716-446655440001", "--yes")
	if err == nil {
		t.Fatal("expected an error when the VM does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
