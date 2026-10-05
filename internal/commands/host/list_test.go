package host

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

const fixtureHosts = `[
	{
		"id": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
		"uuid": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
		"type": "host",
		"name_label": "host-01",
		"address": "10.0.0.11",
		"power_state": "Running",
		"version": "8.2.0",
		"memory": {"size": 2147483648, "usage": 1073741824},
		"cpus": {"cores": 8, "sockets": 2},
		"$pool": "99999999-9999-4999-8999-999999999999",
		"residentVms": ["11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"]
	},
	{
		"id": "aaaaaaaa-bbbb-cccc-dddd-000000000002",
		"uuid": "aaaaaaaa-bbbb-cccc-dddd-000000000002",
		"type": "host",
		"name_label": "host-02",
		"address": "10.0.0.12",
		"power_state": "Halted",
		"version": "8.1.0",
		"memory": {"size": 4294967296, "usage": 2147483648},
		"cpus": {"cores": 4, "sockets": 1},
		"$pool": "99999999-9999-4999-8999-999999999999",
		"residentVms": ["33333333-3333-4333-8333-333333333333"]
	}
]`

// fakeXO serves the minimal REST surface used by Host().GetAll.
func fakeXO(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/hosts" {
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
		_, _ = fmt.Fprint(w, fixtureHosts)
	}))
}

// isolatePointers points the CLI at the fake server and at a throwaway config
// file, and clears any ambient XOA_ variables.
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

func TestHostListTable(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("host list: %v", err)
	}
	for _, expected := range []string{"ID", "NAME", "ADDRESS", "POWER STATE", "VERSION", "MEMORY", "CORES", "SOCKETS", "VMS", "POOL",
		"host-01", "host-02", "10.0.0.11", "10.0.0.12", "Running", "Halted", "8.2.0", "2.147GB", "4.295GB"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
	if strings.Count(out, "host-01") != 1 || strings.Count(out, "host-02") != 1 {
		t.Fatalf("unexpected number of rows:\n%s", out)
	}
}

func TestHostListJSON(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("host list --output json: %v", err)
	}

	var hosts []map[string]any
	if err := json.Unmarshal([]byte(out), &hosts); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0]["name_label"] != "host-01" || hosts[0]["power_state"] != "Running" {
		t.Fatalf("unexpected host payload: %s", out)
	}
}

func TestHostListYAML(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--output", "yaml")
	if err != nil {
		t.Fatalf("host list --output yaml: %v", err)
	}
	if !strings.Contains(out, "name_label: host-01") {
		t.Fatalf("unexpected YAML output:\n%s", out)
	}
}

func TestHostListQuery(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--query", "[?power_state==`Running`].name_label", "--output", "text")
	if err != nil {
		t.Fatalf("host list --query: %v", err)
	}
	if strings.TrimSpace(out) != "host-01" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestHostListQueryInvalid(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--query", "bad["); err == nil {
		t.Fatal("expected an error for an invalid query")
	}
}

func TestHostListLimit(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "3" {
			t.Errorf("expected limit=3, got %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureHosts)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--limit", "3"); err != nil {
		t.Fatalf("host list --limit: %v", err)
	}
}

func TestHostListAPIError(t *testing.T) {
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
	if !strings.Contains(err.Error(), "cannot list hosts") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHostListNoCredentials(t *testing.T) {
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
