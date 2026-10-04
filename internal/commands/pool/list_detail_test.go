package pool

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// listTestPools has two pools with distinct master hosts.
const listTestPools = `[
	{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000001","name_label":"Pool prod","platform_version":"8.2","cpus":{"cores":16,"sockets":2},"master":"11111111-1111-4111-8111-111111111111","HA_enabled":true},
	{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000002","name_label":"Pool lab","platform_version":"8.1","cpus":{"cores":4,"sockets":1},"master":"22222222-2222-4222-8222-222222222222","HA_enabled":false}
]`

// listDetailServer serves the pools collection plus the hosts collection for
// the master names, and records every request path so tests can assert the
// batch cost.
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
		case "/rest/v0/pools":
			_, _ = fmt.Fprint(w, listTestPools)
		case "/rest/v0/hosts":
			_, _ = fmt.Fprint(w, `[
				{"id":"11111111-1111-4111-8111-111111111111","name_label":"master-prod"},
				{"id":"22222222-2222-4222-8222-222222222222","name_label":"master-lab"}
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

// The table shows the master host by name, from a single host batch.
func TestPoolListResolved(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("pool list: %v", err)
	}
	for _, expected := range []string{"master-prod", "master-lab"} {
		if !strings.Contains(out, expected) {
			t.Errorf("list output missing %q:\n%s", expected, out)
		}
	}
	for _, raw := range []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"} {
		if strings.Contains(out, raw) {
			t.Errorf("list output still shows the raw master id %q:\n%s", raw, out)
		}
	}
}

// Every master is resolved with one host batch — never one lookup per pool.
func TestPoolListCostConstant(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	if _, err := runList(t, "list"); err != nil {
		t.Fatalf("pool list: %v", err)
	}
	if n := srv.count("hosts"); n != 1 {
		t.Fatalf("expected exactly 1 host batch, got %d", n)
	}
}

// --output json keeps the raw master reference and makes no batch request.
func TestPoolListJSONNoBatches(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("pool list: %v", err)
	}
	if !strings.Contains(out, `"11111111-1111-4111-8111-111111111111"`) {
		t.Fatalf("json output must keep the raw master reference:\n%s", out)
	}
	if n := srv.count("hosts"); n != 0 {
		t.Fatalf("expected no batch request for json output, got %d", n)
	}
}
