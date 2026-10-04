package vm

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// listTestVMs covers both container cases: VMs in pool mode ($container
// equals $poolId) and a VM pinned to a host (it does not).
const listTestVMs = `[
	{"id":"550e8400-e29b-41d4-a716-446655440001","name_label":"web-01","power_state":"Running","memory":{"size":2147483648},"CPUs":{"number":2},"type":"vm","$poolId":"aaaaaaaa-bbbb-cccc-dddd-000000000009","$container":"aaaaaaaa-bbbb-cccc-dddd-000000000009"},
	{"id":"550e8400-e29b-41d4-a716-446655440002","name_label":"db-01","power_state":"Halted","memory":{"size":4294967296},"CPUs":{"number":4},"type":"vm","$poolId":"aaaaaaaa-bbbb-cccc-dddd-000000000009","$container":"aaaaaaaa-bbbb-cccc-dddd-000000000001"},
	{"id":"550e8400-e29b-41d4-a716-446655440003","name_label":"web-02","power_state":"Halted","memory":{"size":1073741824},"CPUs":{"number":1},"type":"vm","$poolId":"aaaaaaaa-bbbb-cccc-dddd-000000000008","$container":"aaaaaaaa-bbbb-cccc-dddd-000000000008"}
]`

// listDetailServer serves the surface a `vm list` table uses — the VMs
// collection plus the host and pool collections for the container names —
// and records every request path so tests can assert the batch cost.
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
		case r.URL.Path == "/rest/v0/vms":
			_, _ = fmt.Fprint(w, listTestVMs)
		case r.URL.Path == "/rest/v0/hosts":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000001","name_label":"host-01"}
			]`)
		case r.URL.Path == "/rest/v0/pools":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000009","name_label":"prod-pool"},
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000008","name_label":"other-pool"}
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

// The table shows the container by name, using the $poolId field to tell a
// pool-mode VM apart from a VM pinned to a host.
func TestVMListResolved(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("vm list: %v", err)
	}
	for _, expected := range []string{"prod-pool", "host-01", "other-pool"} {
		if !strings.Contains(out, expected) {
			t.Errorf("list output missing %q:\n%s", expected, out)
		}
	}
	// No container UUID should leak into the table.
	for _, raw := range []string{
		"aaaaaaaa-bbbb-cccc-dddd-000000000009",
		"aaaaaaaa-bbbb-cccc-dddd-000000000001",
		"aaaaaaaa-bbbb-cccc-dddd-000000000008",
	} {
		if strings.Contains(out, raw) {
			t.Errorf("list output still shows the raw container id %q:\n%s", raw, out)
		}
	}
}

// Resolving every container must cost one host batch and one pool batch —
// never one lookup per VM, whatever the list size.
func TestVMListCostConstant(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	if _, err := runList(t, "list"); err != nil {
		t.Fatalf("vm list: %v", err)
	}
	if n := srv.count("hosts"); n != 1 {
		t.Fatalf("expected exactly 1 host batch, got %d", n)
	}
	if n := srv.count("pools"); n != 1 {
		t.Fatalf("expected exactly 1 pool batch, got %d", n)
	}
}

// --output json keeps the raw references and must not trigger the batch
// lookups at all: the machine path pays for no extra request.
func TestVMListJSONNoBatches(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("vm list: %v", err)
	}
	if !strings.Contains(out, `"aaaaaaaa-bbbb-cccc-dddd-000000000001"`) {
		t.Fatalf("json output must keep the raw container reference:\n%s", out)
	}
	if n := srv.count("hosts") + srv.count("pools"); n != 0 {
		t.Fatalf("expected no batch request for json output, got %d", n)
	}
}
