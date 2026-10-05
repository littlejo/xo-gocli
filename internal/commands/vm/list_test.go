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

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

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
		"mainIpAddress": "10.0.0.11",
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
	for _, expected := range []string{"ID", "NAME", "POWER STATE", "MEMORY", "CPUS", "IP", "CONTAINER", "web-01", "db-01", "Running", "Halted", "2.147GB", "4.295GB", "10.0.0.11"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
	if strings.Count(out, "web-01") != 1 || strings.Count(out, "db-01") != 1 {
		t.Fatalf("unexpected number of rows:\n%s", out)
	}
}

// TestVMListEmpty pins the S5 empty-table message: an empty list renders the
// header plus "No VMs found." instead of printing nothing.
func TestVMListEmpty(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("vm list (empty): %v", err)
	}
	if !strings.Contains(out, "No VMs found.") {
		t.Fatalf("expected the empty message, got:\n%s", out)
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

// vmState pins the human state: an in-flight operation (from the SDK v1.20.0
// CurrentOperations helpers) wins over the lagging raw power_state, and a
// VM with no current operation shows its raw power_state unchanged.
func TestVMState(t *testing.T) {
	cases := []struct {
		name string
		vm   *payloads.VM
		want string
	}{
		{name: "no operations, running", vm: &payloads.VM{PowerState: "Running"}, want: "Running"},
		{name: "no operations, halted", vm: &payloads.VM{PowerState: "Halted"}, want: "Halted"},
		{
			name: "starting wins over halted",
			vm:   &payloads.VM{PowerState: "Halted", CurrentOperations: map[string]payloads.VMOperation{"t1": payloads.VMOperationStart}},
			want: "Starting",
		},
		{
			name: "shutting down wins over running",
			vm:   &payloads.VM{PowerState: "Running", CurrentOperations: map[string]payloads.VMOperation{"t1": payloads.VMOperationCleanShutdown}},
			want: "Shutting down",
		},
		{
			name: "rebooting wins over halted",
			vm:   &payloads.VM{PowerState: "Halted", CurrentOperations: map[string]payloads.VMOperation{"t1": payloads.VMOperationHardReboot}},
			want: "Rebooting",
		},
		{
			name: "unrelated operation keeps power state",
			vm:   &payloads.VM{PowerState: "Running", CurrentOperations: map[string]payloads.VMOperation{"t1": payloads.VMOperationChangingVCPUsLive}},
			want: "Running",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := vmState(c.vm); got != c.want {
				t.Errorf("vmState = %q, want %q", got, c.want)
			}
		})
	}
}

// A VM whose start is in flight shows "Starting" in the POWER STATE column,
// not the still-lagging "Halted". Machine output is unaffected (raw fields).
func TestVMListTableTransition(t *testing.T) {
	body := `{
		"id": "550e8400-e29b-41d4-a716-446655440001",
		"name_label": "web-01",
		"power_state": "Halted",
		"current_operations": {"t1": "start"},
		"memory": {"size": 2147483648},
		"CPUs": {"number": 2},
		"type": "vm",
		"$container": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	}`
	server := fakeXO(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, "["+body+"]")
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("vm list: %v", err)
	}
	if !strings.Contains(out, "Starting") {
		t.Errorf("expected the POWER STATE column to read Starting:\n%s", out)
	}
	if strings.Contains(out, "Halted") {
		t.Errorf("transition state must not also show the raw power_state:\n%s", out)
	}

	// Machine output still exposes the raw power_state and current_operations.
	jout, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("vm list --output json: %v", err)
	}
	if !strings.Contains(jout, `"power_state": "Halted"`) || !strings.Contains(jout, `"current_operations"`) {
		t.Errorf("json must keep the raw fields:\n%s", jout)
	}
	if strings.Contains(jout, "Starting") {
		t.Errorf("json must not contain the rendered transition state:\n%s", jout)
	}
}
