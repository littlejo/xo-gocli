package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

const fixturePool = `{
	"id": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
	"uuid": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
	"type": "pool",
	"name_label": "Pool prod",
	"platform_version": "8.2",
	"cpus": {"cores": 16, "sockets": 2},
	"master": "11111111-1111-4111-8111-111111111111",
	"HA_enabled": true
}`

// fakeXOGet serves GET /rest/v0/pools/{id}.
func fakeXOGet(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/v0/pools/") {
			http.NotFound(w, r)
			return
		}
		if handler != nil {
			handler(w, r)
			return
		}
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixturePool)
	}))
}

func newGetTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.AddCommand(newGetCommand())
	return root
}

func runGet(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newGetTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

// TestPoolGetDetail checks the human view of `pool get`: it is a detail sheet,
// not the one-row table shared with `pool list`. The fake server serves the
// pool only, so the master falls back to its raw id and the host list is empty
// (the /hosts batch returns 404 and is treated as "no hosts resolvable").
func TestPoolGetDetail(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "aaaaaaaa-bbbb-cccc-dddd-000000000001")
	if err != nil {
		t.Fatalf("pool get: %v", err)
	}
	for _, expected := range []string{
		"Pool prod  (HA)",
		output.DetailField("Master", "11111111-1111-4111-8111-111111111111"),
		output.DetailField("Platform", "8.2"),
		output.DetailField("CPUs", "16 cores, 2 sockets"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

func TestPoolGetJSON(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "aaaaaaaa-bbbb-cccc-dddd-000000000001", "--output", "json")
	if err != nil {
		t.Fatalf("pool get --output json: %v", err)
	}

	var pool map[string]any
	if err := json.Unmarshal([]byte(out), &pool); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if pool["name_label"] != "Pool prod" || pool["platform_version"] != "8.2" {
		t.Fatalf("unexpected pool payload: %s", out)
	}
}

func TestPoolGetQuery(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "aaaaaaaa-bbbb-cccc-dddd-000000000001", "--query", "name_label")
	if err != nil {
		t.Fatalf("pool get --query: %v", err)
	}
	if strings.TrimSpace(out) != "Pool prod" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestPoolGetNotFound(t *testing.T) {
	server := fakeXOGet(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"not found"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runGet(t, "get", "aaaaaaaa-bbbb-cccc-dddd-000000000001")
	if err == nil {
		t.Fatal("expected an error when the pool does not exist")
	}
	if !strings.Contains(err.Error(), `pool "aaaaaaaa-bbbb-cccc-dddd-000000000001" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoolGetAPIError(t *testing.T) {
	server := fakeXOGet(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"boom"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runGet(t, "get", "aaaaaaaa-bbbb-cccc-dddd-000000000001")
	if err == nil {
		t.Fatal("expected an error when the API fails")
	}
	if !strings.Contains(err.Error(), "cannot get pool") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoolGetBadID(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runGet(t, "get", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
}

const (
	poolTestID    = "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	poolMasterID  = "11111111-1111-4111-8111-111111111111"
	poolDefaultSR = "aaaaaaaa-bbbb-cccc-dddd-000000000002"
)

// poolDetailServer serves the full surface a `pool get` detail view uses and
// records every request path so tests can assert the resolver's cost. The pool
// has master host-master, default SR "Local Storage" and two member hosts
// (plus one host from another pool, which must be filtered out).
//
//	GET /rest/v0/pools/{id} -> the pool
//	GET /rest/v0/hosts/{id} -> host-master (the master, when resolveNames)
//	GET /rest/v0/hosts      -> all hosts, for the batch member listing
//	GET /rest/v0/srs/{id}   -> Local Storage (when resolveNames)
func poolDetailServer(t *testing.T, resolveNames bool) (*httptest.Server, *[]string) {
	t.Helper()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/rest/v0/pools/"):
			_, _ = fmt.Fprint(w, `{"id":"`+poolTestID+`","name_label":"Pool prod","platform_version":"8.2","master":"`+poolMasterID+`","default_SR":"`+poolDefaultSR+`","cpus":{"cores":16,"sockets":2}}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/hosts/"):
			if resolveNames {
				_, _ = fmt.Fprint(w, `{"id":"`+poolMasterID+`","name_label":"host-master"}`)
			} else {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"not found"}`)
			}
		case r.URL.Path == "/rest/v0/hosts":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-00000000000a","name_label":"host-01","$pool":"`+poolTestID+`"},
				{"id":"aaaaaaaa-bbbb-cccc-dddd-00000000000b","name_label":"host-02","$pool":"`+poolTestID+`"},
				{"id":"aaaaaaaa-bbbb-cccc-dddd-00000000000c","name_label":"other-pool-host","$pool":"00000000-0000-0000-0000-000000000009"}
			]`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/srs/"):
			if resolveNames {
				_, _ = fmt.Fprint(w, `{"name_label":"Local Storage"}`)
			} else {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"not found"}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &paths
}

func countPaths(paths *[]string, prefix string) int {
	p := "/rest/v0/" + prefix
	n := 0
	for _, x := range *paths {
		if strings.HasPrefix(x, p) {
			n++
		}
	}
	return n
}

// The detail sheet resolves the master and the default SR by name, and lists
// the pool's member hosts by name (filtering out hosts of other pools).
func TestPoolGetDetailResolved(t *testing.T) {
	srv, _ := poolDetailServer(t, true)
	isolatePointers(t, srv.URL)

	out, err := runGet(t, "get", poolTestID)
	if err != nil {
		t.Fatalf("pool get: %v", err)
	}
	for _, expected := range []string{
		output.DetailField("Master", "host-master"),
		output.DetailField("Default SR", "Local Storage"),
		output.DetailField("Hosts", "host-01, host-02"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
	if strings.Contains(out, "other-pool-host") {
		t.Errorf("a host from another pool must not be listed:\n%s", out)
	}
}

// Resolving the member hosts must cost one host batch (not one lookup per
// host) plus the single master lookup — a constant cost.
func TestPoolGetDetailCostConstant(t *testing.T) {
	srv, paths := poolDetailServer(t, true)
	isolatePointers(t, srv.URL)

	if _, err := runGet(t, "get", poolTestID); err != nil {
		t.Fatalf("pool get: %v", err)
	}
	// One bare /rest/v0/hosts batch (never one per host) and one /hosts/{id}
	// for the master: 2 host requests total for a 2-host pool.
	if n := countPaths(paths, "hosts"); n != 2 {
		t.Fatalf("expected 2 host requests (1 batch + 1 master), got %d", n)
	}
	if n := countPaths(paths, "srs/"); n != 1 {
		t.Fatalf("expected 1 SR lookup (default SR), got %d", n)
	}
}
