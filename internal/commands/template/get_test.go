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
	"github.com/littlejo/xo-gocli/internal/output"
)

const fixtureTemplate = `{
	"id": "d31e47fd-a70e-d849-883e-c17193472710-6959dfe8-534c-4c58-8a8c-3c3792293543",
	"uuid": "6959dfe8-534c-4c58-8a8c-3c3792293543",
	"type": "VM-template",
	"name_label": "Oracle Linux 8",
	"isDefaultTemplate": true,
	"power_state": "Halted",
	"memory": {"size": 4294967296},
	"CPUs": {"number": 2, "max": 2},
	"$pool": "d31e47fd-a70e-d849-883e-c17193472710"
}`

// templateID is the (composite) id of fixtureTemplate.
const templateID = "d31e47fd-a70e-d849-883e-c17193472710-6959dfe8-534c-4c58-8a8c-3c3792293543"

// fakeXOGet serves GET /rest/v0/vm-templates/{id}.
func fakeXOGet(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/v0/vm-templates/") {
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
		_, _ = fmt.Fprint(w, fixtureTemplate)
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

// TestTemplateGetDetail checks the human view of `template get`: it is a
// detail sheet, not the one-row table shared with `template list`. The fake
// server serves the template only, so the pool falls back to its raw id.
func TestTemplateGetDetail(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", templateID)
	if err != nil {
		t.Fatalf("template get: %v", err)
	}
	for _, expected := range []string{
		"Template Oracle Linux 8  (default)",
		output.DetailField("Pool", "d31e47fd-a70e-d849-883e-c17193472710"),
		output.DetailField("Memory", "4.295GB"),
		output.DetailField("CPUs", "2"),
		output.DetailField("Power state", "Halted"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

func TestTemplateGetJSON(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", templateID, "--output", "json")
	if err != nil {
		t.Fatalf("template get --output json: %v", err)
	}
	var tmpl map[string]any
	if err := json.Unmarshal([]byte(out), &tmpl); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if tmpl["name_label"] != "Oracle Linux 8" {
		t.Fatalf("unexpected template payload: %s", out)
	}
}

func TestTemplateGetQuery(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "get", templateID, "--query", "name_label", "--output", "text")
	if err != nil {
		t.Fatalf("template get --query: %v", err)
	}
	if strings.TrimSpace(out) != "Oracle Linux 8" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestTemplateGetNotFound(t *testing.T) {
	server := fakeXOGet(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"not found"}`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runGet(t, "get", templateID)
	if err == nil {
		t.Fatal("expected an error when the template does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemplateGetBadID(t *testing.T) {
	server := fakeXOGet(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runGet(t, "get", "a/b"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
}

// The detail sheet resolves the pool by name.
func TestTemplateGetDetailResolved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/rest/v0/vm-templates/"):
			_, _ = fmt.Fprint(w, fixtureTemplate)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/pools/"):
			_, _ = fmt.Fprint(w, `{"name_label":"prod-pool"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	isolatePointers(t, srv.URL)

	out, err := runGet(t, "get", templateID)
	if err != nil {
		t.Fatalf("template get: %v", err)
	}
	if !strings.Contains(out, output.DetailField("Pool", "prod-pool")) {
		t.Fatalf("expected the resolved pool name:\n%s", out)
	}
	if !strings.Contains(out, "Template Oracle Linux 8  (default)") {
		t.Fatalf("expected the template header:\n%s", out)
	}
}
