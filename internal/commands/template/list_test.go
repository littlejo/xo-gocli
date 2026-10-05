package template

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

// fixtureTemplates is the shape returned by GET /rest/v0/vm-templates.
const fixtureTemplates = `[
	{
		"id": "d31e47fd-a70e-d849-883e-c17193472710-6959dfe8-534c-4c58-8a8c-3c3792293543",
		"uuid": "6959dfe8-534c-4c58-8a8c-3c3792293543",
		"type": "VM-template",
		"name_label": "Oracle Linux 8",
		"isDefaultTemplate": true,
		"power_state": "Halted",
		"memory": {"size": 4294967296},
		"CPUs": {"number": 2, "max": 2},
		"$pool": "d31e47fd-a70e-d849-883e-c17193472710"
	},
	{
		"id": "d31e47fd-a70e-d849-883e-c17193472710-7aa32be8-a06c-4ade-8a1d-49e51e03e9d2",
		"uuid": "7aa32be8-a06c-4ade-8a1d-49e51e03e9d2",
		"type": "VM-template",
		"name_label": "AlmaLinux 8",
		"isDefaultTemplate": true,
		"power_state": "Halted",
		"memory": {"size": 4294967296},
		"CPUs": {"number": 1, "max": 1},
		"$pool": "d31e47fd-a70e-d849-883e-c17193472710"
	},
	{
		"id": "d31e47fd-a70e-d849-883e-c17193472710-552bce37-51b2-445d-84f2-5f33fa112d7e",
		"uuid": "552bce37-51b2-445d-84f2-5f33fa112d7e",
		"type": "VM-template",
		"name_label": "Other install media",
		"isDefaultTemplate": false,
		"power_state": "Halted",
		"memory": {"size": 280909312},
		"CPUs": {"number": 1, "max": 1},
		"$pool": "d31e47fd-a70e-d849-883e-c17193472710"
	}
]`

// fakeXO serves GET /rest/v0/vm-templates, the dedicated templates resource.
// It records the query string so tests can assert on fields/limit.
func fakeXO(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/vm-templates" {
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
		_, _ = fmt.Fprint(w, fixtureTemplates)
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

func TestTemplateListTable(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}
	for _, expected := range []string{"ID", "NAME", "DEFAULT", "MEMORY", "CPUS", "POOL", "Oracle Linux 8", "AlmaLinux 8", "Other install media", "4.295GB", "true", "false"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
	if strings.Count(out, "Oracle Linux 8") != 1 || strings.Count(out, "AlmaLinux 8") != 1 {
		t.Fatalf("unexpected number of rows:\n%s", out)
	}
}

func TestTemplateListUsesTemplatesResource(t *testing.T) {
	var gotFields string
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		gotFields = r.URL.Query().Get("fields")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTemplates)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list"); err != nil {
		t.Fatalf("template list: %v", err)
	}
	if gotFields != "*" {
		t.Fatalf("expected fields=*, got %q", gotFields)
	}
}

func TestTemplateListJSON(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("template list --output json: %v", err)
	}

	var vms []map[string]any
	if err := json.Unmarshal([]byte(out), &vms); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(vms) != 3 {
		t.Fatalf("expected 3 templates, got %d:\n%s", len(vms), out)
	}
	for _, vm := range vms {
		if vm["type"] != "VM-template" {
			t.Fatalf("unexpected template payload: %s", out)
		}
	}
}

func TestTemplateListQuery(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list", "--query", "[?isDefaultTemplate].name_label", "--output", "text")
	if err != nil {
		t.Fatalf("template list --query: %v", err)
	}
	// The two default templates are kept, the non-default one is filtered out.
	if !strings.Contains(out, "Oracle Linux 8") || !strings.Contains(out, "AlmaLinux 8") || strings.Contains(out, "Other install media") {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestTemplateListQueryInvalid(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--query", "bad["); err == nil {
		t.Fatal("expected an error for an invalid query")
	}
}

func TestTemplateListLimit(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "2" {
			t.Errorf("expected limit=2, got %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureTemplates)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runList(t, "list", "--limit", "2"); err != nil {
		t.Fatalf("template list --limit: %v", err)
	}
}

func TestTemplateListEmpty(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("template list (empty): %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected no rows, got:\n%s", out)
	}
}

func TestTemplateListAPIError(t *testing.T) {
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
	if !strings.Contains(err.Error(), "cannot list templates") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemplateListNoCredentials(t *testing.T) {
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
