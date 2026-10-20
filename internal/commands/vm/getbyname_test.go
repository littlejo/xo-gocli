package vm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/testenv"
)

// byNameServer answers the name lookup (GET /rest/v0/vms?filter=...) with
// list, and the VM read with the detail fixture; it records the filters.
func byNameServer(t *testing.T, list string, filters *[]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/v0/vms":
			*filters = append(*filters, r.URL.Query().Get("filter"))
			_, _ = fmt.Fprint(w, list)
		case "/rest/v0/vms/" + detailVMID:
			_, _ = fmt.Fprint(w, detailFixtureVM)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	testenv.Isolate(t, srv.URL)
	return srv
}

func runGetByName(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "xo", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	vmCmd := &cobra.Command{Use: "vm"}
	vmCmd.AddCommand(newGetCommand())
	root.AddCommand(vmCmd)
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestGetByName(t *testing.T) {
	var filters []string
	byNameServer(t, `[{"id":"`+detailVMID+`","name_label":"web-01"},{"id":"550e8400-e29b-41d4-a716-446655440009","name_label":"web-010"}]`, &filters)

	out, err := runGetByName(t, "vm", "get", "web-01", "--output", "json")
	if err != nil {
		t.Fatalf("vm get web-01: %v", err)
	}
	if !strings.Contains(out, `"id": "`+detailVMID+`"`) {
		t.Fatalf("expected the web-01 VM:\n%s", out)
	}
	if len(filters) != 1 || filters[0] != `name_label:"web-01"` {
		t.Fatalf("filters = %q", filters)
	}
}

func TestGetByNameAmbiguous(t *testing.T) {
	var filters []string
	byNameServer(t, `[{"id":"`+detailVMID+`","name_label":"web"},{"id":"550e8400-e29b-41d4-a716-446655440009","name_label":"web"}]`, &filters)

	_, err := runGetByName(t, "vm", "get", "web")
	if err == nil || !strings.Contains(err.Error(), `2 VMs are named "web"`) {
		t.Fatalf("expected an ambiguity error, got %v", err)
	}
}

func TestGetByNameNotFound(t *testing.T) {
	var filters []string
	byNameServer(t, `[]`, &filters)

	_, err := runGetByName(t, "vm", "get", "nope")
	if err == nil || err.Error() != `VM "nope" not found` {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}
