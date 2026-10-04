package vm

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// vdisListServer serves the surface a `vm vdis` table uses — the per-VM VDI
// listing (existence check included) plus the SR collection for the names —
// and records every request path so tests can assert the batch cost.
type vdisListServer struct {
	*httptest.Server
	paths []string
}

func newVDISListServer(t *testing.T) *vdisListServer {
	t.Helper()
	s := &vdisListServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.paths = append(s.paths, r.URL.Path)
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/vdis"):
			_, _ = fmt.Fprint(w, fixtureVDIs)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			_, _ = fmt.Fprint(w, fixtureVM)
		case r.URL.Path == "/rest/v0/srs":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000001","name_label":"Local Storage"}
			]`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *vdisListServer) count(prefix string) int {
	p := "/rest/v0/" + prefix
	n := 0
	for _, x := range s.paths {
		if strings.HasPrefix(x, p) {
			n++
		}
	}
	return n
}

// The table shows the SR each VDI lives on by name, from a single SR batch.
func TestVMVdisResolved(t *testing.T) {
	srv := newVDISListServer(t)
	isolateVM(t, srv.URL)

	out, err := runVM(t, "vm", "vdis", "550e8400-e29b-41d4-a716-446655440001")
	if err != nil {
		t.Fatalf("vm vdis: %v", err)
	}
	if !strings.Contains(out, "Local Storage") {
		t.Fatalf("list output missing the SR name:\n%s", out)
	}
	if strings.Contains(out, "aaaaaaaa-bbbb-cccc-dddd-000000000001") {
		t.Fatalf("list output still shows the raw SR id:\n%s", out)
	}
	if n := srv.count("srs"); n != 1 {
		t.Fatalf("expected exactly 1 SR batch, got %d", n)
	}
}

// --output json keeps the raw SR reference and makes no SR batch request.
func TestVMVdisJSONNoBatches(t *testing.T) {
	srv := newVDISListServer(t)
	isolateVM(t, srv.URL)

	out, err := runVM(t, "vm", "vdis", "550e8400-e29b-41d4-a716-446655440001", "--output", "json")
	if err != nil {
		t.Fatalf("vm vdis --output json: %v", err)
	}
	if !strings.Contains(out, `"aaaaaaaa-bbbb-cccc-dddd-000000000001"`) {
		t.Fatalf("json output must keep the raw SR reference:\n%s", out)
	}
	if n := srv.count("srs"); n != 0 {
		t.Fatalf("expected no batch request for json output, got %d", n)
	}
}
