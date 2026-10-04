package pbd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// listTestPBDs share a host and pool, and span two SRs.
const listTestPBDs = `[
	{"id":"66666666-6666-4666-8666-666666666666","type":"PBD","$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009","attached":true,"host":"aaaaaaaa-bbbb-cccc-dddd-000000000001","SR":"aaaaaaaa-bbbb-cccc-dddd-000000000002","device_config":{"device":"/dev/sda"}},
	{"id":"66666666-6666-4666-8666-666666666667","type":"PBD","$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009","attached":false,"host":"aaaaaaaa-bbbb-cccc-dddd-000000000001","SR":"aaaaaaaa-bbbb-cccc-dddd-000000000003","device_config":{"server":"nfs-host","serverpath":"/export"}}
]`

// listDetailServer serves the PBD collection plus the host, SR and pool
// collections for the names, and records every request path so tests can
// assert the batch cost.
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
		case "/rest/v0/pbds":
			_, _ = fmt.Fprint(w, listTestPBDs)
		case "/rest/v0/hosts":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000001","name_label":"host-01"}
			]`)
		case "/rest/v0/srs":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000002","name_label":"Local Storage"},
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000003","name_label":"NFS Share"}
			]`)
		case "/rest/v0/pools":
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

// The table shows the host, SR and pool each PBD connects by name.
func TestPBDListResolved(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runPBD(t, "pbd", "list")
	if err != nil {
		t.Fatalf("pbd list: %v", err)
	}
	for _, expected := range []string{"host-01", "Local Storage", "NFS Share", "prod-pool"} {
		if !strings.Contains(out, expected) {
			t.Errorf("list output missing %q:\n%s", expected, out)
		}
	}
	for _, raw := range []string{
		"aaaaaaaa-bbbb-cccc-dddd-000000000001",
		"aaaaaaaa-bbbb-cccc-dddd-000000000002",
		"aaaaaaaa-bbbb-cccc-dddd-000000000003",
		"aaaaaaaa-bbbb-cccc-dddd-000000000009",
	} {
		if strings.Contains(out, raw) {
			t.Errorf("list output still shows the raw reference %q:\n%s", raw, out)
		}
	}
}

// Every host, SR and pool is resolved with one batch each — never one lookup
// per PBD.
func TestPBDListCostConstant(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	if _, err := runPBD(t, "pbd", "list"); err != nil {
		t.Fatalf("pbd list: %v", err)
	}
	if n := srv.count("hosts"); n != 1 {
		t.Fatalf("expected exactly 1 host batch, got %d", n)
	}
	if n := srv.count("srs"); n != 1 {
		t.Fatalf("expected exactly 1 SR batch, got %d", n)
	}
	if n := srv.count("pools"); n != 1 {
		t.Fatalf("expected exactly 1 pool batch, got %d", n)
	}
}

// --output json keeps the raw references and makes no batch request.
func TestPBDListJSONNoBatches(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runPBD(t, "pbd", "list", "--output", "json")
	if err != nil {
		t.Fatalf("pbd list --output json: %v", err)
	}
	if !strings.Contains(out, `"aaaaaaaa-bbbb-cccc-dddd-000000000002"`) {
		t.Fatalf("json output must keep the raw SR reference:\n%s", out)
	}
	if n := srv.count("hosts") + srv.count("srs") + srv.count("pools"); n != 0 {
		t.Fatalf("expected no batch request for json output, got %d", n)
	}
}
