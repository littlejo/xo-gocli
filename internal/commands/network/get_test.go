package network

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

const fixtureNetwork = `{
	"id": "11111111-1111-4111-8111-111111111111",
	"uuid": "11111111-1111-4111-8111-111111111111",
	"type": "network",
	"name_label": "Management",
	"bridge": "xenbr0",
	"MTU": 1500,
	"automatic": true,
	"defaultIsLocked": true,
	"isBonded": false,
	"VIFs": [
		"aaaaaaaa-bbbb-cccc-dddd-000000000001",
		"aaaaaaaa-bbbb-cccc-dddd-000000000002"
	],
	"$pool": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
}`

// fakeXOGet serves GET /rest/v0/networks/{id}.
func fakeXOGet(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/v0/networks/") {
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
		_, _ = fmt.Fprint(w, fixtureNetwork)
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

// TestNetworkGetDetail checks the human view of `network get`: it is a detail
// sheet, not the one-row table shared with `network list`. The fake server
// serves the network only, so the pool falls back to its raw id.
func TestNetworkGetDetail(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatalf("network get: %v", err)
	}
	for _, expected := range []string{
		"Network Management",
		output.DetailField("Pool", "aaaaaaaa-bbbb-cccc-dddd-000000000001"),
		output.DetailField("Bridge", "xenbr0"),
		output.DetailField("MTU", "1500"),
		output.DetailField("VIFs", "2"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

func TestNetworkGetJSON(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111", "--output", "json")
	if err != nil {
		t.Fatalf("network get --output json: %v", err)
	}

	var network map[string]any
	if err := json.Unmarshal([]byte(out), &network); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if network["name_label"] != "Management" || network["bridge"] != "xenbr0" {
		t.Fatalf("unexpected network payload: %s", out)
	}
}

func TestNetworkGetQuery(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111", "--query", "name_label")
	if err != nil {
		t.Fatalf("network get --query: %v", err)
	}
	if strings.TrimSpace(out) != "Management" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestNetworkGetNotFound(t *testing.T) {
	server := fakeXOGet(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"not found"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runGet(t, "get", "11111111-1111-4111-8111-111111111111")
	if err == nil {
		t.Fatal("expected an error when the network does not exist")
	}
	if !strings.Contains(err.Error(), "network \"11111111-1111-4111-8111-111111111111\" not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNetworkGetAPIError(t *testing.T) {
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
	if !strings.Contains(err.Error(), "cannot get network") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNetworkGetBadID(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runGet(t, "get", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
}

const (
	networkTestID = "11111111-1111-4111-8111-111111111111"
	networkPoolID = "aaaaaaaa-bbbb-cccc-dddd-000000000001"
)

// networkDetailServer serves the surface a `network get` detail view uses and
// records every request path so tests can assert the resolver's cost. The
// network belongs to a pool.
//
//	GET /rest/v0/networks/{id} -> the network
//	GET /rest/v0/pools/{id}    -> prod-pool (when resolveNames)
func networkDetailServer(t *testing.T, resolveNames bool) (*httptest.Server, *[]string) {
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
		case strings.HasPrefix(r.URL.Path, "/rest/v0/networks/"):
			_, _ = fmt.Fprint(w, `{"id":"`+networkTestID+`","name_label":"Management","bridge":"xenbr0","MTU":1500,"type":"external","VIFs":["aaaaaaaa-bbbb-cccc-dddd-000000000001","aaaaaaaa-bbbb-cccc-dddd-000000000002"],"PIFs":["bbbbbbbb-bbbb-cccc-dddd-000000000001","bbbbbbbb-bbbb-cccc-dddd-000000000002","bbbbbbbb-bbbb-cccc-dddd-000000000003"],"$pool":"`+networkPoolID+`"}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/pools/"):
			if resolveNames {
				_, _ = fmt.Fprint(w, `{"name_label":"prod-pool"}`)
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

func countNetworkPaths(paths *[]string, prefix string) int {
	p := "/rest/v0/" + prefix
	n := 0
	for _, x := range *paths {
		if strings.HasPrefix(x, p) {
			n++
		}
	}
	return n
}

// The detail sheet resolves the pool by name and shows the VIF/PIF counts.
func TestNetworkGetDetailResolved(t *testing.T) {
	srv, paths := networkDetailServer(t, true)
	isolatePointers(t, srv.URL)

	out, err := runGet(t, "get", networkTestID)
	if err != nil {
		t.Fatalf("network get: %v", err)
	}
	for _, expected := range []string{
		"Network Management  (external)",
		output.DetailField("Pool", "prod-pool"),
		output.DetailField("Bridge", "xenbr0"),
		output.DetailField("VIFs", "2"),
		output.DetailField("PIFs", "3"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
	// One pool lookup for the single relationship, nothing else.
	if n := countNetworkPaths(paths, "pools/"); n != 1 {
		t.Fatalf("expected exactly 1 pool lookup, got %d", n)
	}
}

// An unresolvable pool must not fail the command: the raw id is shown.
func TestNetworkGetDetailFallback(t *testing.T) {
	srv, _ := networkDetailServer(t, false)
	isolatePointers(t, srv.URL)

	out, err := runGet(t, "get", networkTestID)
	if err != nil {
		t.Fatalf("network get must succeed even when the pool is missing: %v", err)
	}
	if !strings.Contains(out, output.DetailField("Pool", networkPoolID)) {
		t.Fatalf("expected the raw pool id as fallback:\n%s", out)
	}
}
