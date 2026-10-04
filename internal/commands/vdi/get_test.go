package vdi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	vdiVM1 = "550e8400-e29b-41d4-a716-446655440001"
	vdiVM2 = "550e8400-e29b-41d4-a716-446655440002"
)

// detailServer serves the surface a `vdi get` detail view uses and records
// every request path so tests can assert the resolver's cost. The fixture VDI
// lives on SR ...001 and is attached (via VBDs) to vdiVM1 and vdiVM2.
//
//	GET /rest/v0/vdis/{id}  -> fixtureVDI (only for getVDIID)
//	GET /rest/v0/srs/{id}   -> Local Storage (when resolveSR)
//	GET /rest/v0/vbds       -> the VBDs attached to the VDI (when listVBDs)
//	GET /rest/v0/vms        -> all VMs, for the batch name lookup
type detailServer struct {
	*httptest.Server
	paths []string
}

func newDetailServer(t *testing.T, resolveSR, listVBDs bool) *detailServer {
	t.Helper()
	s := &detailServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.paths = append(s.paths, r.URL.Path)
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/rest/v0/vdis/"):
			if r.URL.Path == "/rest/v0/vdis/"+getVDIID {
				_, _ = fmt.Fprint(w, fixtureVDI)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/srs/"):
			if resolveSR {
				_, _ = fmt.Fprint(w, `{"name_label":"Local Storage"}`)
			} else {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"not found"}`)
			}
		case r.URL.Path == "/rest/v0/vbds":
			if listVBDs {
				_, _ = fmt.Fprint(w, `[
					{"id":"33333333-3333-4333-8333-333333333333","VDI":"`+getVDIID+`","VM":"`+vdiVM1+`"},
					{"id":"44444444-4444-4444-8444-444444444444","VDI":"`+getVDIID+`","VM":"`+vdiVM2+`"}
				]`)
			} else {
				_, _ = fmt.Fprint(w, `[]`)
			}
		case r.URL.Path == "/rest/v0/vms":
			_, _ = fmt.Fprint(w, `[
				{"id":"`+vdiVM1+`","name_label":"web-01"},
				{"id":"`+vdiVM2+`","name_label":"web-02"}
			]`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *detailServer) count(prefix string) int {
	p := "/rest/v0/" + prefix
	n := 0
	for _, x := range s.paths {
		if strings.HasPrefix(x, p) {
			n++
		}
	}
	return n
}

// The detail sheet resolves the SR by name and lists the attaching VMs by name.
func TestVDIGetDetailResolved(t *testing.T) {
	srv := newDetailServer(t, true, true)
	isolatePointers(t, srv.URL)

	out, err := runGet(t, "vdi", "get", getVDIID)
	if err != nil {
		t.Fatalf("vdi get: %v", err)
	}
	for _, expected := range []string{
		"VDI system disk",
		output.DetailField("SR", "Local Storage"),
		output.DetailField("Type", "system"),
		output.DetailField("Attached to", "web-01, web-02"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

// Resolving the attaching VMs must cost one VBD list and one VM batch — never
// one VM lookup per attaching VM (anti-N+1).
func TestVDIGetDetailCostConstant(t *testing.T) {
	srv := newDetailServer(t, true, true)
	isolatePointers(t, srv.URL)

	if _, err := runGet(t, "vdi", "get", getVDIID); err != nil {
		t.Fatalf("vdi get: %v", err)
	}
	if n := srv.count("vbds"); n != 1 {
		t.Fatalf("expected exactly 1 VBD list, got %d", n)
	}
	// The batch fetches all VMs in a single call, not one per attaching VM.
	if n := srv.count("vms"); n != 1 {
		t.Fatalf("expected exactly 1 VM batch (not one per VM), got %d", n)
	}
}

// A relationship that cannot be resolved must not fail the command: the raw id
// is shown instead, and an unresolvable "attached to" is simply omitted.
func TestVDIGetDetailFallback(t *testing.T) {
	srv := newDetailServer(t, false, false)
	isolatePointers(t, srv.URL)

	out, err := runGet(t, "vdi", "get", getVDIID)
	if err != nil {
		t.Fatalf("vdi get must succeed even when the relationships are missing: %v", err)
	}
	if !strings.Contains(out, output.DetailField("SR", "aaaaaaaa-bbbb-cccc-dddd-000000000001")) {
		t.Fatalf("expected the raw SR id as fallback:\n%s", out)
	}
}
