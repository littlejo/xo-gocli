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
)

const fixtureNetworks = `[
	{
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
	},
	{
		"id": "22222222-2222-4222-8222-222222222222",
		"uuid": "22222222-2222-4222-8222-222222222222",
		"type": "network",
		"name_label": "VMs",
		"bridge": "xenbr1",
		"MTU": 9000,
		"automatic": false,
		"defaultIsLocked": false,
		"isBonded": true,
		"VIFs": [
			"aaaaaaaa-bbbb-cccc-dddd-000000000003"
		],
		"$pool": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	}
]`

// fakeXO serves the minimal REST surface used by Network().GetAll.
func fakeXO(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/networks" {
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
		_, _ = fmt.Fprint(w, fixtureNetworks)
	}))
}

func isolatePointers(t *testing.T, url string) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE", "XOA_YES"} {
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

func TestNetworkListTable(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("network list: %v", err)
	}
	for _, expected := range []string{"ID", "NAME", "BRIDGE", "TYPE", "MTU", "VIFS", "POOL", "Management", "VMs", "xenbr0", "xenbr1", "1500", "9000"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
}

func TestNetworkListJSON(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("network list --output json: %v", err)
	}

	var networks []map[string]any
	if err := json.Unmarshal([]byte(out), &networks); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(networks) != 2 {
		t.Fatalf("expected 2 networks, got %d", len(networks))
	}
	if networks[0]["name_label"] != "Management" || networks[0]["bridge"] != "xenbr0" {
		t.Fatalf("unexpected network payload: %s", out)
	}
}

func TestNetworkListQuery(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--query", "[?isBonded].name_label", "--output", "text")
	if err != nil {
		t.Fatalf("network list --query: %v", err)
	}
	if strings.TrimSpace(out) != "VMs" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestNetworkListQueryInvalid(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--query", "bad["); err == nil {
		t.Fatal("expected an error for an invalid query")
	}
}

func TestNetworkListLimit(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "3" {
			t.Errorf("expected limit=3, got %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureNetworks)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--limit", "3"); err != nil {
		t.Fatalf("network list --limit: %v", err)
	}
}

func TestNetworkListAPIError(t *testing.T) {
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
	if !strings.Contains(err.Error(), "cannot list networks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNetworkListNoCredentials(t *testing.T) {
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
