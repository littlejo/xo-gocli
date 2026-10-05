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
)

const fixtureSRs = `[
	{
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
	},
	{
		"id": "22222222-2222-4222-8222-222222222222",
		"uuid": "22222222-2222-4222-8222-222222222222",
		"type": "SR",
		"name_label": "NFS data",
		"SR_type": "nfs",
		"size": 1099511627776,
		"usage": 214748364800,
		"physical_usage": 190000000000,
		"content_type": "user",
		"shared": true,
		"$container": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	}
]`

// fakeXO serves the minimal REST surface used by SR().GetAll.
func fakeXO(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/srs" {
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
		_, _ = fmt.Fprint(w, fixtureSRs)
	}))
}

func isolatePointers(t *testing.T, url string) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE", "XOA_WAIT"} {
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

func TestSRListTable(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("sr list: %v", err)
	}
	for _, expected := range []string{"ID", "NAME", "TYPE", "SIZE", "USAGE", "Local storage", "NFS data", "lvm", "nfs", "10.74GB", "1.1TB", "214.7GB"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
}

func TestSRListJSON(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("sr list --output json: %v", err)
	}

	var srs []map[string]any
	if err := json.Unmarshal([]byte(out), &srs); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(srs) != 2 {
		t.Fatalf("expected 2 SRs, got %d", len(srs))
	}
	if srs[0]["name_label"] != "Local storage" || srs[0]["SR_type"] != "lvm" {
		t.Fatalf("unexpected SR payload: %s", out)
	}
}

func TestSRListQuery(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--query", "[?SR_type==`nfs`].name_label", "--output", "text")
	if err != nil {
		t.Fatalf("sr list --query: %v", err)
	}
	if strings.TrimSpace(out) != "NFS data" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestSRListQueryInvalid(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--query", "bad["); err == nil {
		t.Fatal("expected an error for an invalid query")
	}
}

func TestSRListType(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter") != "SR_type:lvm" {
			t.Errorf("expected filter=SR_type:lvm, got %q", r.URL.Query().Get("filter"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureSRs)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--type", "lvm"); err != nil {
		t.Fatalf("sr list --type: %v", err)
	}
}

func TestSRListLimit(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "3" {
			t.Errorf("expected limit=3, got %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureSRs)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--limit", "3"); err != nil {
		t.Fatalf("sr list --limit: %v", err)
	}
}

func TestSRListAPIError(t *testing.T) {
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
	if !strings.Contains(err.Error(), "cannot list storage repositories") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSRListNoCredentials(t *testing.T) {
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
