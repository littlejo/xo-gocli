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
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	detailVMID         = "550e8400-e29b-41d4-a716-446655440001"
	detailHostID       = "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	detailPoolID       = "cccccccc-bbbb-cccc-dddd-000000000001"
	detailTemplateUUID = "6959dfe8-534c-4c58-8a8c-3c3792293543"
	// detailTemplateID is the composite template id the REST API uses
	// (<poolId>-<templateUuid>); it is what `resolve.Template` expects.
	detailTemplateID = detailPoolID + "-" + detailTemplateUUID
	detailFixtureVM  = `{
		"id": "` + detailVMID + `",
		"uuid": "` + detailVMID + `",
		"type": "vm",
		"name_label": "web-01",
		"name_description": "primary web server",
		"power_state": "Running",
		"memory": {"size": 4294967296},
		"CPUs": {"number": 2, "max": 4},
		"boot": {"firmware": "hvm", "order": "cda"},
		"tags": ["production", "web"],
		"mainIpAddress": "10.0.0.12",
		"virtualizationMode": "hvm",
		"$VBDs": ["11111111-1111-4111-8111-111111111111"],
		"VIFs": ["22222222-2222-4222-8222-222222222222"],
		"snapshots": ["33333333-3333-4333-8333-333333333333"],
		"auto_poweron": true,
		"template": "` + detailTemplateUUID + `",
		"$poolId": "` + detailPoolID + `",
		"$container": "` + detailHostID + `",
		"creation": {"date": "2026-01-02T10:00:00Z", "user": "admin"}
	}`
)

// detailServer serves the full surface a `vm get` detail view uses and records
// every request path so tests can assert the resolver's cost:
//
//	GET /rest/v0/vms/{id}                 -> the fixture VM
//	GET /rest/v0/hosts/{id}               -> host-02 (when containerIsHost)
//	GET /rest/v0/pools/{id}               -> 404 (never expected for a host)
//	GET /rest/v0/vm-templates/{id}        -> the template (unless template404)
type detailServer struct {
	*httptest.Server
	paths []string
}

func newDetailServer(t *testing.T, containerIsHost, template404 bool) *detailServer {
	t.Helper()
	s := &detailServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.paths = append(s.paths, r.URL.Path)
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			_, _ = fmt.Fprint(w, detailFixtureVM)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/hosts/"):
			if containerIsHost {
				_, _ = fmt.Fprint(w, `{"id":"`+detailHostID+`","name_label":"host-02"}`)
			} else {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"object not found"}`)
			}
		case strings.HasPrefix(r.URL.Path, "/rest/v0/pools/"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"object not found"}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/vm-templates/"):
			if template404 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"object not found"}`)
			} else {
				_, _ = fmt.Fprint(w, `{"name_label":"Oracle Linux 8"}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Server.Close)
	return s
}

func (s *detailServer) count(prefix string) int {
	p := "/rest/v0/" + prefix
	n := 0
	for _, x := range s.paths {
		if strings.HasPrefix(x, p) {
			n++
		}
	}
	return n
}

func newGetTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.AddCommand(NewCommand())
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

func isolateGet(t *testing.T, url string) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE"} {
		t.Setenv(key, "")
	}
	t.Setenv("XOA_ENDPOINT", url)
	t.Setenv("XOA_TOKEN", "test-token")
}

// The detail sheet must show the raw fields of the object plus the resolved
// relationships (container by name, template by name).
func TestVMGetDetailResolved(t *testing.T) {
	srv := newDetailServer(t, true, false)
	isolateGet(t, srv.URL)

	out, err := runGet(t, "vm", "get", detailVMID)
	if err != nil {
		t.Fatalf("vm get: %v", err)
	}
	for _, expected := range []string{
		"VM web-01  (Running)",
		output.DetailField("IP", "10.0.0.12"),
		output.DetailField("Description", "primary web server"),
		output.DetailField("Tags", "production, web"),
		output.DetailField("Container", "host-02"),
		output.DetailField("Template", "Oracle Linux 8"),
		output.DetailField("Memory", "4.295GB"),
		output.DetailField("CPUs", "2 (max 4)"),
		output.DetailField("Disks", "1  (xo vm vdis "+detailVMID+")"),
		output.DetailField("Boot", "hvm, order cda"),
		output.DetailField("Flags", "hvm auto-poweron"),
		output.DetailField("Created", "2026-01-02T10:00:00Z by admin"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
	// Counts are derived, not resolved: no per-VBD/VIF request.
	if n := srv.count("vbds/"); n != 0 {
		t.Errorf("no VBD request expected, got %d", n)
	}
}

// The container resolution must cost at most one host and one pool request, and
// the template at most one. This is the anti-N+1 contract: a constant number of
// extra lookups, independent of how large the pool is.
func TestVMGetDetailCostConstant(t *testing.T) {
	srv := newDetailServer(t, true, false)
	isolateGet(t, srv.URL)

	if _, err := runGet(t, "vm", "get", detailVMID); err != nil {
		t.Fatalf("vm get: %v", err)
	}
	if n := srv.count("hosts/"); n != 1 {
		t.Fatalf("expected exactly 1 host lookup, got %d", n)
	}
	// The container is a host, so the pool is never queried.
	if n := srv.count("pools/"); n != 0 {
		t.Fatalf("expected 0 pool lookups for a host container, got %d", n)
	}
	if n := srv.count("vm-templates/"); n != 1 {
		t.Fatalf("expected exactly 1 template lookup, got %d", n)
	}
}

// When the container is not a host, the resolver falls back to the pool.
func TestVMGetDetailContainerIsPool(t *testing.T) {
	// A container that is a pool: hosts/ 404s, pools/ answers.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			_, _ = fmt.Fprint(w, detailFixtureVM)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/hosts/"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"object not found"}`)
		case strings.HasPrefix(r.URL.Path, "/rest/v0/pools/"):
			_, _ = fmt.Fprint(w, `{"id":"`+detailHostID+`","name_label":"prod-pool"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	isolateGet(t, srv.URL)

	out, err := runGet(t, "vm", "get", detailVMID)
	if err != nil {
		t.Fatalf("vm get: %v", err)
	}
	if !strings.Contains(out, output.DetailField("Container", "prod-pool")) {
		t.Fatalf("expected the container to resolve to the pool name:\n%s", out)
	}
}

// A relationship that cannot be resolved must not fail the command: the raw id
// is shown instead.
func TestVMGetDetailTemplateMissingFallsBack(t *testing.T) {
	srv := newDetailServer(t, true, true)
	isolateGet(t, srv.URL)

	out, err := runGet(t, "vm", "get", detailVMID)
	if err != nil {
		t.Fatalf("vm get must succeed even when the template is missing: %v", err)
	}
	if !strings.Contains(out, output.DetailField("Template", detailTemplateID)) {
		t.Fatalf("expected the raw template id as fallback:\n%s", out)
	}
}
