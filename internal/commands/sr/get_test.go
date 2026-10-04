package sr

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

const fixtureSR = `{
	"id": "11111111-1111-4111-8111-111111111111",
	"uuid": "11111111-1111-4111-8111-111111111111",
	"type": "SR",
	"name_label": "Local storage",
	"SR_type": "lvm",
	"size": 10737418240,
	"usage": 5368709120,
	"physical_usage": 3221225472,
	"content_type": "user",
	"shared": false,
	"$container": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
}`

// fakeXOGet serves GET /rest/v0/srs/{id}.
func fakeXOGet(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/v0/srs/") {
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
		_, _ = fmt.Fprint(w, fixtureSR)
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

// TestSRGetDetail checks the human view of `sr get`: it is a detail sheet, not
// the one-row table shared with `sr list`. The fake server does not serve
// hosts/pools or PBDs, so the container falls back to its raw id and the
// "Connected to" line is omitted (the fixture SR has no PBDs).
func TestSRGetDetail(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatalf("sr get: %v", err)
	}
	for _, expected := range []string{
		"SR Local storage",
		output.DetailField("Type", "lvm"),
		output.DetailField("Content", "user"),
		output.DetailField("Container", "aaaaaaaa-bbbb-cccc-dddd-000000000001"),
		output.DetailField("Size", "10.74GB"),
		output.DetailField("Usage", "5.369GB / 10.74GB (50%)"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

func TestSRGetJSON(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111", "--output", "json")
	if err != nil {
		t.Fatalf("sr get --output json: %v", err)
	}

	var sr map[string]any
	if err := json.Unmarshal([]byte(out), &sr); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if sr["name_label"] != "Local storage" || sr["SR_type"] != "lvm" {
		t.Fatalf("unexpected SR payload: %s", out)
	}
}

func TestSRGetQuery(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111", "--query", "name_label")
	if err != nil {
		t.Fatalf("sr get --query: %v", err)
	}
	if strings.TrimSpace(out) != "Local storage" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestSRGetNotFound(t *testing.T) {
	server := fakeXOGet(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"not found"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111")
	if err == nil {
		t.Fatal("expected an error when the SR does not exist")
	}
	if !strings.Contains(err.Error(), `SR "11111111-1111-4111-8111-111111111111" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSRGetAPIError(t *testing.T) {
	server := fakeXOGet(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"boom"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111")
	if err == nil {
		t.Fatal("expected an error when the API fails")
	}
	if !strings.Contains(err.Error(), "cannot get SR") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSRGetBadID(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runGet(t, "get", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
}

// srDetailServer serves the full surface a `sr get` detail view uses and
// records every request path so tests can assert the resolver's cost. The SR
// is a pool container, is shared and has two PBDs plugged into hostA/hostB.
//
//	GET /rest/v0/srs/{id}   -> the SR
//	GET /rest/v0/hosts/{id} -> 404 (the container is a pool, not a host)
//	GET /rest/v0/pools/{id} -> prod-pool
//	GET /rest/v0/pbds       -> the PBDs attached to the SR (when listPBDs)
//	GET /rest/v0/hosts      -> all hosts, for the batch name lookup
func srDetailServer(t *testing.T, listPBDs bool) (*httptest.Server, *[]string) {
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
		case strings.HasPrefix(r.URL.Path, "/rest/v0/srs/"):
			_, _ = fmt.Fprint(w, `{
				"id": "11111111-1111-4111-8111-111111111111",
				"name_label": "Local storage",
				"SR_type": "lvm",
				"size": 10737418240,
				"usage": 5368709120,
				"shared": true,
				"allocationStrategy": "thin",
				"tags": ["prod"],
				"$container": "aaaaaaaa-bbbb-cccc-dddd-000000000009",
				"VDIs": [],
				"$PBDs": ["aaaaaaaa-bbbb-cccc-dddd-00000000000a", "aaaaaaaa-bbbb-cccc-dddd-00000000000b"]
			}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/hosts/"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/pools/"):
			_, _ = fmt.Fprint(w, `{"name_label":"prod-pool"}`)
		case r.URL.Path == "/rest/v0/pbds":
			if listPBDs {
				_, _ = fmt.Fprint(w, `[
					{"id":"aaaaaaaa-bbbb-cccc-dddd-00000000000a","SR":"11111111-1111-4111-8111-111111111111","host":"aaaaaaaa-bbbb-cccc-dddd-000000000001"},
					{"id":"aaaaaaaa-bbbb-cccc-dddd-00000000000b","SR":"11111111-1111-4111-8111-111111111111","host":"aaaaaaaa-bbbb-cccc-dddd-000000000002"}
				]`)
			} else {
				_, _ = fmt.Fprint(w, `[]`)
			}
		case r.URL.Path == "/rest/v0/hosts":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000001","name_label":"host-01"},
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000002","name_label":"host-02"}
			]`)
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

// The detail sheet resolves the container by name and lists the connected
// hosts by name.
func TestSRGetDetailResolved(t *testing.T) {
	srv, _ := srDetailServer(t, true)
	isolatePointers(t, srv.URL)

	out, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatalf("sr get: %v", err)
	}
	for _, expected := range []string{
		"SR Local storage",
		output.DetailField("Container", "prod-pool"),
		output.DetailField("Shared", "yes"),
		output.DetailField("Allocation", "thin"),
		output.DetailField("Connected to", "host-01, host-02"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

// Resolving the connected hosts must cost one PBD list and one host batch —
// never one host lookup per PBD (anti-N+1).
func TestSRGetDetailCostConstant(t *testing.T) {
	srv, paths := srDetailServer(t, true)
	isolatePointers(t, srv.URL)

	if _, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatalf("sr get: %v", err)
	}
	if n := countPaths(paths, "pbds"); n != 1 {
		t.Fatalf("expected exactly 1 PBD list, got %d", n)
	}
	// The batch fetches all hosts in a single call, not one per PBD. Along
	// with the container's host lookup (a 404) that is 2 calls to /hosts,
	// plus the pool lookup.
	if n := countPaths(paths, "hosts"); n != 2 {
		t.Fatalf("expected 2 host requests (1 container lookup + 1 batch), got %d", n)
	}
	if n := countPaths(paths, "pools/"); n != 1 {
		t.Fatalf("expected exactly 1 pool lookup, got %d", n)
	}
}
