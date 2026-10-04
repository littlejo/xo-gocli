package network

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// listTestNetworks both belong to the same pool.
const listTestNetworks = `[
	{"id":"11111111-1111-4111-8111-111111111111","name_label":"Management","bridge":"xenbr0","type":"network","MTU":1500,"VIFs":["aaaaaaaa-bbbb-cccc-dddd-000000000001"],"PIFs":["bbbbbbbb-bbbb-cccc-dddd-000000000001"],"$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009"},
	{"id":"22222222-2222-4222-8222-222222222222","name_label":"Storage","bridge":"xenbr1","type":"network","MTU":9000,"VIFs":[],"PIFs":["bbbbbbbb-bbbb-cccc-dddd-000000000002","bbbbbbbb-bbbb-cccc-dddd-000000000003"],"$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009"}
]`

// listDetailServer serves the networks collection plus the pools collection
// for the pool names, and records every request path so tests can assert the
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
		switch {
		case r.URL.Path == "/rest/v0/networks":
			_, _ = fmt.Fprint(w, listTestNetworks)
		case r.URL.Path == "/rest/v0/pools":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000009","name_label":"prod-pool"}
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

// The table shows the pool each network belongs to by name.
func TestNetworkListResolved(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("network list: %v", err)
	}
	if !strings.Contains(out, "prod-pool") {
		t.Fatalf("list output missing the pool name:\n%s", out)
	}
	if strings.Contains(out, "aaaaaaaa-bbbb-cccc-dddd-000000000009") {
		t.Fatalf("list output still shows the raw pool id:\n%s", out)
	}
}

// The single pool is resolved with one batch — never one lookup per network.
func TestNetworkListCostConstant(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	if _, err := runList(t, "list"); err != nil {
		t.Fatalf("network list: %v", err)
	}
	if n := srv.count("pools"); n != 1 {
		t.Fatalf("expected exactly 1 pool batch, got %d", n)
	}
}

// --output json keeps the raw pool reference and makes no batch request.
func TestNetworkListJSONNoBatches(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("network list: %v", err)
	}
	if !strings.Contains(out, `"aaaaaaaa-bbbb-cccc-dddd-000000000009"`) {
		t.Fatalf("json output must keep the raw pool reference:\n%s", out)
	}
	if n := srv.count("pools"); n != 0 {
		t.Fatalf("expected no batch request for json output, got %d", n)
	}
}
