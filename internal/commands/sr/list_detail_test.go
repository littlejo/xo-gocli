package sr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// listTestSRs covers both container cases: an SR whose container is its pool
// ($container equals $pool) and an SR whose container is a host.
const listTestSRs = `[
	{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000001","name_label":"Local storage","type":"SR","SR_type":"lvm","size":10737418240,"usage":5368709120,"$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009","$container":"aaaaaaaa-bbbb-cccc-dddd-000000000009"},
	{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000002","name_label":"NFS Share","type":"SR","SR_type":"nfs","size":21474836480,"usage":10737418240,"$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009","$container":"bbbbbbbb-bbbb-cccc-dddd-000000000001"}
]`

// listDetailServer serves the SRs collection plus the host and pool
// collections for the container names, and records every request path so
// tests can assert the batch cost.
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
		case "/rest/v0/srs":
			_, _ = fmt.Fprint(w, listTestSRs)
		case "/rest/v0/pools":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000009","name_label":"prod-pool"}
			]`)
		case "/rest/v0/hosts":
			_, _ = fmt.Fprint(w, `[
				{"id":"bbbbbbbb-bbbb-cccc-dddd-000000000001","name_label":"host-01"}
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

// The table shows the container by name, using the $pool field to tell a
// pool-owned SR apart from an SR whose container is a host.
func TestSRListResolved(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("sr list: %v", err)
	}
	for _, expected := range []string{"prod-pool", "host-01"} {
		if !strings.Contains(out, expected) {
			t.Errorf("list output missing %q:\n%s", expected, out)
		}
	}
	for _, raw := range []string{
		"aaaaaaaa-bbbb-cccc-dddd-000000000009",
		"bbbbbbbb-bbbb-cccc-dddd-000000000001",
	} {
		if strings.Contains(out, raw) {
			t.Errorf("list output still shows the raw container id %q:\n%s", raw, out)
		}
	}
}

// Every container is resolved with one host batch and one pool batch — never
// one lookup per SR.
func TestSRListCostConstant(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	if _, err := runList(t, "list"); err != nil {
		t.Fatalf("sr list: %v", err)
	}
	if n := srv.count("pools"); n != 1 {
		t.Fatalf("expected exactly 1 pool batch, got %d", n)
	}
	if n := srv.count("hosts"); n != 1 {
		t.Fatalf("expected exactly 1 host batch, got %d", n)
	}
}

// --output json keeps the raw container reference and makes no batch request.
func TestSRListJSONNoBatches(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("sr list: %v", err)
	}
	if !strings.Contains(out, `"aaaaaaaa-bbbb-cccc-dddd-000000000009"`) {
		t.Fatalf("json output must keep the raw container reference:\n%s", out)
	}
	if n := srv.count("pools") + srv.count("hosts"); n != 0 {
		t.Fatalf("expected no batch request for json output, got %d", n)
	}
}
