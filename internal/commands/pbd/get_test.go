package pbd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/output"
)

// detailServer serves the surface a `pbd get` detail view uses and records
// every request path so tests can assert the resolver's cost. The fixture PBD
// points at pbdHostID / pbdSRID / pbdPoolID.
//
//	GET /rest/v0/pbds/{id}  -> fixturePBD (only for getPBDID)
//	GET /rest/v0/hosts/{id} -> host-01 (when resolve)
//	GET /rest/v0/srs/{id}   -> NFS Share (when resolve)
//	GET /rest/v0/pools/{id} -> prod-pool (when resolve)
type detailServer struct {
	*httptest.Server
	paths []string
}

func newDetailServer(t *testing.T, resolve bool) *detailServer {
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
		case strings.HasPrefix(r.URL.Path, "/rest/v0/pbds/"):
			if r.URL.Path == "/rest/v0/pbds/"+getPBDID {
				_, _ = fmt.Fprint(w, fixturePBD)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/hosts/"):
			writeResolved(w, r, resolve, `{"name_label":"host-01"}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/srs/"):
			writeResolved(w, r, resolve, `{"name_label":"NFS Share"}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/pools/"):
			writeResolved(w, r, resolve, `{"name_label":"prod-pool"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func writeResolved(w http.ResponseWriter, r *http.Request, resolve bool, body string) {
	if resolve {
		_, _ = fmt.Fprint(w, body)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	_, _ = fmt.Fprint(w, `{"message":"not found"}`)
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

// The detail sheet resolves host, SR and pool by name.
func TestPBDGetDetailResolved(t *testing.T) {
	srv := newDetailServer(t, true)
	isolatePointers(t, srv.URL)

	out, err := runPBD(t, "pbd", "get", getPBDID)
	if err != nil {
		t.Fatalf("pbd get: %v", err)
	}
	for _, expected := range []string{
		"PBD /dev/sda  (attached=yes)",
		output.DetailField("Host", "host-01"),
		output.DetailField("SR", "NFS Share"),
		output.DetailField("Pool", "prod-pool"),
		output.DetailField("Config", "device=/dev/sda"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

// The resolution must cost exactly one host, one SR and one pool lookup — a
// constant number of extra requests, never one per field (anti-N+1).
func TestPBDGetDetailCostConstant(t *testing.T) {
	srv := newDetailServer(t, true)
	isolatePointers(t, srv.URL)

	if _, err := runPBD(t, "pbd", "get", getPBDID); err != nil {
		t.Fatalf("pbd get: %v", err)
	}
	if n := srv.count("hosts/"); n != 1 {
		t.Fatalf("expected exactly 1 host lookup, got %d", n)
	}
	if n := srv.count("srs/"); n != 1 {
		t.Fatalf("expected exactly 1 SR lookup, got %d", n)
	}
	if n := srv.count("pools/"); n != 1 {
		t.Fatalf("expected exactly 1 pool lookup, got %d", n)
	}
}

// A relationship that cannot be resolved must not fail the command: the raw id
// is shown instead.
func TestPBDGetDetailFallback(t *testing.T) {
	srv := newDetailServer(t, false)
	isolatePointers(t, srv.URL)

	out, err := runPBD(t, "pbd", "get", getPBDID)
	if err != nil {
		t.Fatalf("pbd get must succeed even when the relationships are missing: %v", err)
	}
	for _, expected := range []string{
		output.DetailField("Host", pbdHostID),
		output.DetailField("SR", pbdSRID),
		output.DetailField("Pool", pbdPoolID),
	} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected the raw id as fallback, missing %q:\n%s", expected, out)
		}
	}
}
