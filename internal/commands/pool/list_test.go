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
)

const fixturePools = `[
	{
		"id": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
		"uuid": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
		"type": "pool",
		"name_label": "Pool prod",
		"platform_version": "8.2",
		"cpus": {"cores": 16, "sockets": 2},
		"master": "11111111-1111-4111-8111-111111111111",
		"HA_enabled": true
	},
	{
		"id": "aaaaaaaa-bbbb-cccc-dddd-000000000002",
		"uuid": "aaaaaaaa-bbbb-cccc-dddd-000000000002",
		"type": "pool",
		"name_label": "Pool lab",
		"platform_version": "8.1",
		"cpus": {"cores": 4, "sockets": 1},
		"master": "22222222-2222-4222-8222-222222222222",
		"HA_enabled": false
	}
]`

// fakeXO serves the minimal REST surface used by Pool().GetAll.
func fakeXO(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/pools" {
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
		_, _ = fmt.Fprint(w, fixturePools)
	}))
}

func isolatePointers(t *testing.T, url string) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE"} {
		t.Setenv(key, "")
	}
	t.Setenv("XOA_ENDPOINT", url)
	t.Setenv("XOA_TOKEN", "test-token")
}

func newTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	root.AddCommand(newListCommand())
	return root
}

func runList(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestPoolListTable(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("pool list: %v", err)
	}
	for _, expected := range []string{"ID", "NAME", "VERSION", "CORES", "SOCKETS", "MASTER", "HA", "Pool prod", "Pool lab", "8.2", "16", "yes", "no"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
}

func TestPoolListJSON(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("pool list --output json: %v", err)
	}

	var pools []map[string]any
	if err := json.Unmarshal([]byte(out), &pools); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(pools) != 2 {
		t.Fatalf("expected 2 pools, got %d", len(pools))
	}
	if pools[0]["name_label"] != "Pool prod" {
		t.Fatalf("unexpected pool payload: %s", out)
	}
}

func TestPoolListQuery(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--query", "[?HA_enabled].name_label", "--output", "text")
	if err != nil {
		t.Fatalf("pool list --query: %v", err)
	}
	if strings.TrimSpace(out) != "Pool prod" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestPoolListQueryInvalid(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--query", "bad["); err == nil {
		t.Fatal("expected an error for an invalid query")
	}
}

func TestPoolListLimit(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "3" {
			t.Errorf("expected limit=3, got %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixturePools)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--limit", "3"); err != nil {
		t.Fatalf("pool list --limit: %v", err)
	}
}

func TestPoolListAPIError(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"boom"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runList(t, "list")
	if err == nil {
		t.Fatal("expected an error when the API fails")
	}
	if !strings.Contains(err.Error(), "cannot list pools") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoolListNoCredentials(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE"} {
		t.Setenv(key, "")
	}

	_, err := runList(t, "list")
	if err == nil {
		t.Fatal("expected an error when nothing is configured")
	}
	if !strings.Contains(err.Error(), "no endpoint configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}
