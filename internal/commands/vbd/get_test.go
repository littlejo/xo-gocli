package vbd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/output"
)

// getVBD's relationships (from fixtureVBD): VM = vmID, VDI = getVBDVDIID.
const getVBDVDIID = "11111111-1111-4111-8111-111111111111"

// detailServer serves the surface a `vbd get` detail view uses and records
// every request path so tests can assert the resolver's cost:
//
//	GET /rest/v0/vbds/{id}  -> fixtureVBD (only for getVBDID)
//	GET /rest/v0/vms/{id}   -> web-01 (when resolveVM)
//	GET /rest/v0/vdis/{id}  -> sys-disk 2GiB (when resolveVDI)
type detailServer struct {
	*httptest.Server
	paths []string
}

func newDetailServer(t *testing.T, resolveVM, resolveVDI bool) *detailServer {
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
		case strings.HasPrefix(r.URL.Path, "/rest/v0/vbds/"):
			if r.URL.Path == "/rest/v0/vbds/"+getVBDID {
				_, _ = fmt.Fprint(w, fixtureVBD)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			if resolveVM {
				_, _ = fmt.Fprint(w, `{"name_label":"web-01"}`)
			} else {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"not found"}`)
			}
		case strings.HasPrefix(r.URL.Path, "/rest/v0/vdis/"):
			if resolveVDI {
				_, _ = fmt.Fprint(w, `{"name_label":"sys-disk","size":2147483648}`)
			} else {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"not found"}`)
			}
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

// The detail sheet resolves the VM by name and the VDI by name+size.
func TestVBDGetDetailResolved(t *testing.T) {
	srv := newDetailServer(t, true, true)
	isolatePointers(t, srv.URL)

	out, err := runVBD(t, "vbd", "get", getVBDID)
	if err != nil {
		t.Fatalf("vbd get: %v", err)
	}
	for _, expected := range []string{
		"VBD xvda  (RW, attached=yes)",
		output.DetailField("VM", "web-01"),
		output.DetailField("VDI", "sys-disk"),
		output.DetailField("Size", "2.147GB"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

// The resolution must cost exactly one VM and one VDI lookup — a constant
// number of extra requests, never one per field (anti-N+1).
func TestVBDGetDetailCostConstant(t *testing.T) {
	srv := newDetailServer(t, true, true)
	isolatePointers(t, srv.URL)

	if _, err := runVBD(t, "vbd", "get", getVBDID); err != nil {
		t.Fatalf("vbd get: %v", err)
	}
	if n := srv.count("vms/"); n != 1 {
		t.Fatalf("expected exactly 1 VM lookup, got %d", n)
	}
	if n := srv.count("vdis/"); n != 1 {
		t.Fatalf("expected exactly 1 VDI lookup, got %d", n)
	}
}

// A relationship that cannot be resolved must not fail the command: the raw id
// is shown instead.
func TestVBDGetDetailFallback(t *testing.T) {
	srv := newDetailServer(t, false, false)
	isolatePointers(t, srv.URL)

	out, err := runVBD(t, "vbd", "get", getVBDID)
	if err != nil {
		t.Fatalf("vbd get must succeed even when the relationships are missing: %v", err)
	}
	if !strings.Contains(out, output.DetailField("VM", vmID)) {
		t.Fatalf("expected the raw VM id as fallback:\n%s", out)
	}
	if !strings.Contains(out, output.DetailField("VDI", getVBDVDIID)) {
		t.Fatalf("expected the raw VDI id as fallback:\n%s", out)
	}
}
