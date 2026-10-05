package vm

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

const fixtureVMs = `[
	{
		"id": "550e8400-e29b-41d4-a716-446655440001",
		"name_label": "web-01",
		"power_state": "Running",
		"memory": {"size": 2147483648},
		"CPUs": {"number": 2},
		"boot": {"order": "cd"},
		"type": "vm",
		"$container": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	},
	{
		"id": "550e8400-e29b-41d4-a716-446655440002",
		"name_label": "db-01",
		"power_state": "Halted",
		"memory": {"size": 4294967296},
		"CPUs": {"number": 4},
		"boot": {"order": "c"},
		"type": "vm",
		"$container": "aaaaaaaa-bbbb-cccc-dddd-000000000002"
	}
]`

// fakeXO serves the minimal REST surface used by VM().GetAll.
func fakeXO(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/vms" {
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
		_, _ = fmt.Fprint(w, fixtureVMs)
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

func TestVMListTable(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("vm list: %v", err)
	}
	for _, expected := range []string{"ID", "NAME", "POWER STATE", "MEMORY", "CPUS", "web-01", "db-01", "Running", "Halted", "2.147GB", "4.295GB"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
	if strings.Count(out, "web-01") != 1 || strings.Count(out, "db-01") != 1 {
		t.Fatalf("unexpected number of rows:\n%s", out)
	}
}

func TestVMListJSON(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("vm list --output json: %v", err)
	}

	var vms []map[string]any
	if err := json.Unmarshal([]byte(out), &vms); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(vms) != 2 {
		t.Fatalf("expected 2 VMs, got %d", len(vms))
	}
	if vms[0]["name_label"] != "web-01" || vms[0]["power_state"] != "Running" {
		t.Fatalf("unexpected VM payload: %s", out)
	}
}

func TestVMListYAML(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--output", "yaml")
	if err != nil {
		t.Fatalf("vm list --output yaml: %v", err)
	}
	if !strings.Contains(out, "name_label: web-01") {
		t.Fatalf("unexpected YAML output:\n%s", out)
	}
}

func TestVMListQuery(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--query", "[?power_state==`Running`].name_label", "--output", "text")
	if err != nil {
		t.Fatalf("vm list --query: %v", err)
	}
	if strings.TrimSpace(out) != "web-01" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestVMListQueryInvalid(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--query", "bad["); err == nil {
		t.Fatal("expected an error for an invalid query")
	}
}

func TestVMListLimit(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "3" {
			t.Errorf("expected limit=3, got %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureVMs)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--limit", "3"); err != nil {
		t.Fatalf("vm list --limit: %v", err)
	}
}

func TestVMListPowerState(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter") != "power_state:Running" {
			t.Errorf("expected filter=power_state:Running, got %q", r.URL.Query().Get("filter"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureVMs)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--power-state", "Running"); err != nil {
		t.Fatalf("vm list --power-state: %v", err)
	}
}

func TestVMListAPIError(t *testing.T) {
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
	if !strings.Contains(err.Error(), "cannot list VMs") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVMListNoCredentials(t *testing.T) {
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

func TestVMListBadEndpoint(t *testing.T) {
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE"} {
		t.Setenv(key, "")
	}
	t.Setenv("XOA_ENDPOINT", "://invalid")
	t.Setenv("XOA_TOKEN", "test-token")

	_, err := runList(t, "list")
	if err == nil {
		t.Fatal("expected an error for an invalid endpoint")
	}
}
