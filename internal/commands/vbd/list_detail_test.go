package vbd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// listTestVBDs share one VM, span two VDIs, and include a CD drive (no VDI).
const listTestVBDs = `[
	{"id":"33333333-3333-4333-8333-333333333333","type":"VBD","$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009","attached":true,"device":"xvda","position":"0","VDI":"11111111-1111-4111-8111-111111111111","VM":"550e8400-e29b-41d4-a716-446655440001"},
	{"id":"44444444-4444-4444-8444-444444444444","type":"VBD","$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009","attached":false,"device":"xvdb","position":"1","VDI":"22222222-2222-4222-8222-222222222222","VM":"550e8400-e29b-41d4-a716-446655440001"},
	{"id":"55555555-5555-4555-8555-555555555555","type":"VBD","$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009","attached":false,"device":"cd0","position":"xvdd","VM":"550e8400-e29b-41d4-a716-446655440001"}
]`

// listDetailServer serves the VBD collection plus the VM and VDI collections
// for the names, and records every request path so tests can assert the batch
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
		case "/rest/v0/vbds":
			_, _ = fmt.Fprint(w, listTestVBDs)
		case "/rest/v0/vms":
			_, _ = fmt.Fprint(w, `[
				{"id":"550e8400-e29b-41d4-a716-446655440001","name_label":"web-01"}
			]`)
		case "/rest/v0/vdis":
			_, _ = fmt.Fprint(w, `[
				{"id":"11111111-1111-4111-8111-111111111111","name_label":"sys-disk"},
				{"id":"22222222-2222-4222-8222-222222222222","name_label":"data-disk"}
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

// The table shows the attached VM and VDI by name; a VBD without a VDI (a CD
// drive) keeps its dash.
func TestVBDListResolved(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runVBD(t, "vbd", "list")
	if err != nil {
		t.Fatalf("vbd list: %v", err)
	}
	for _, expected := range []string{"web-01", "sys-disk", "data-disk"} {
		if !strings.Contains(out, expected) {
			t.Errorf("list output missing %q:\n%s", expected, out)
		}
	}
	for _, raw := range []string{
		"550e8400-e29b-41d4-a716-446655440001",
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
	} {
		if strings.Contains(out, raw) {
			t.Errorf("list output still shows the raw reference %q:\n%s", raw, out)
		}
	}
	// The CD drive row keeps its empty VDI cell.
	if !strings.Contains(out, "-") {
		t.Fatalf("expected a dash for the VBD without a VDI:\n%s", out)
	}
}

// Every VM and VDI is resolved with one batch each — never one lookup per
// VBD.
func TestVBDListCostConstant(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	if _, err := runVBD(t, "vbd", "list"); err != nil {
		t.Fatalf("vbd list: %v", err)
	}
	if n := srv.count("vms"); n != 1 {
		t.Fatalf("expected exactly 1 VM batch, got %d", n)
	}
	if n := srv.count("vdis"); n != 1 {
		t.Fatalf("expected exactly 1 VDI batch, got %d", n)
	}
}

// --output json keeps the raw references and makes no batch request.
func TestVBDListJSONNoBatches(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runVBD(t, "vbd", "list", "--output", "json")
	if err != nil {
		t.Fatalf("vbd list --output json: %v", err)
	}
	if !strings.Contains(out, `"550e8400-e29b-41d4-a716-446655440001"`) {
		t.Fatalf("json output must keep the raw VM reference:\n%s", out)
	}
	if n := srv.count("vms") + srv.count("vdis"); n != 0 {
		t.Fatalf("expected no batch request for json output, got %d", n)
	}
}
