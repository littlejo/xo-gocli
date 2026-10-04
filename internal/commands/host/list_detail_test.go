package host

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// listTestHosts belongs to two distinct pools.
const listTestHosts = `[
	{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000001","name_label":"host-01","address":"10.0.0.11","power_state":"Running","version":"8.2.0","memory":{"size":2147483648,"usage":1073741824},"cpus":{"cores":8,"sockets":2},"$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009"},
	{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000002","name_label":"host-02","address":"10.0.0.12","power_state":"Halted","version":"8.2.0","memory":{"size":4294967296,"usage":2147483648},"cpus":{"cores":4,"sockets":1},"$pool":"bbbbbbbb-bbbb-cccc-dddd-000000000009"}
]`

// listDetailServer serves the hosts collection plus the pools collection for
// the pool names, and records every request path so tests can assert the
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
		case "/rest/v0/hosts":
			_, _ = fmt.Fprint(w, listTestHosts)
		case "/rest/v0/pools":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000009","name_label":"prod-pool"},
				{"id":"bbbbbbbb-bbbb-cccc-dddd-000000000009","name_label":"lab-pool"}
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

// The table shows the pool each host belongs to by name.
func TestHostListResolved(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("host list: %v", err)
	}
	for _, expected := range []string{"prod-pool", "lab-pool"} {
		if !strings.Contains(out, expected) {
			t.Errorf("list output missing %q:\n%s", expected, out)
		}
	}
	for _, raw := range []string{
		"aaaaaaaa-bbbb-cccc-dddd-000000000009",
		"bbbbbbbb-bbbb-cccc-dddd-000000000009",
	} {
		if strings.Contains(out, raw) {
			t.Errorf("list output still shows the raw pool id %q:\n%s", raw, out)
		}
	}
}

// Every pool is resolved with a single batch — never one lookup per host.
func TestHostListCostConstant(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	if _, err := runList(t, "list"); err != nil {
		t.Fatalf("host list: %v", err)
	}
	if n := srv.count("pools"); n != 1 {
		t.Fatalf("expected exactly 1 pool batch, got %d", n)
	}
}

// --output json keeps the raw pool reference and makes no batch request.
func TestHostListJSONNoBatches(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("host list: %v", err)
	}
	if !strings.Contains(out, `"aaaaaaaa-bbbb-cccc-dddd-000000000009"`) {
		t.Fatalf("json output must keep the raw pool reference:\n%s", out)
	}
	if n := srv.count("pools"); n != 0 {
		t.Fatalf("expected no batch request for json output, got %d", n)
	}
}
