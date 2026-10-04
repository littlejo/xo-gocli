package vdi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// listTestVDIs all live on the same SR.
const listTestVDIs = `[
	{"id":"11111111-1111-4111-8111-111111111111","name_label":"system disk","type":"VDI","VDI_type":"system","size":21474836480,"usage":10737418240,"$SR":"aaaaaaaa-bbbb-cccc-dddd-000000000002","$poolId":"aaaaaaaa-bbbb-cccc-dddd-000000000009"},
	{"id":"22222222-2222-4222-8222-222222222222","name_label":"data disk","type":"VDI","VDI_type":"user","size":10737418240,"usage":5368709120,"$SR":"aaaaaaaa-bbbb-cccc-dddd-000000000002","$poolId":"aaaaaaaa-bbbb-cccc-dddd-000000000009"}
]`

// listDetailServer serves the VDI collection plus the SR collection for the
// SR names, and records every request path so tests can assert the batch
// cost.
type listDetailServer struct {
	*httptest.Server
	paths []string
}

func newListDetailServer(t *testing.T) *listDetailServer {
	t.Helper()
	s := &listDetailServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.paths = append(s.paths, r.URL.Path)
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/v0/vdis":
			_, _ = fmt.Fprint(w, listTestVDIs)
		case "/rest/v0/srs":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000002","name_label":"Local Storage"}
			]`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *listDetailServer) count(prefix string) int {
	p := "/rest/v0/" + prefix
	n := 0
	for _, x := range s.paths {
		if strings.HasPrefix(x, p) {
			n++
		}
	}
	return n
}

// The table shows the SR each VDI lives on by name.
func TestVDIListResolved(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runVDI(t, "vdi", "list")
	if err != nil {
		t.Fatalf("vdi list: %v", err)
	}
	if !strings.Contains(out, "Local Storage") {
		t.Fatalf("list output missing the SR name:\n%s", out)
	}
	if strings.Contains(out, "aaaaaaaa-bbbb-cccc-dddd-000000000002") {
		t.Fatalf("list output still shows the raw SR id:\n%s", out)
	}
}

// Every SR is resolved with a single batch — never one lookup per VDI.
func TestVDIListCostConstant(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	if _, err := runVDI(t, "vdi", "list"); err != nil {
		t.Fatalf("vdi list: %v", err)
	}
	if n := srv.count("srs"); n != 1 {
		t.Fatalf("expected exactly 1 SR batch, got %d", n)
	}
}

// --output json keeps the raw SR reference and makes no batch request.
func TestVDIListJSONNoBatches(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runVDI(t, "vdi", "list", "--output", "json")
	if err != nil {
		t.Fatalf("vdi list --output json: %v", err)
	}
	if !strings.Contains(out, `"aaaaaaaa-bbbb-cccc-dddd-000000000002"`) {
		t.Fatalf("json output must keep the raw SR reference:\n%s", out)
	}
	if n := srv.count("srs"); n != 0 {
		t.Fatalf("expected no batch request for json output, got %d", n)
	}
}
