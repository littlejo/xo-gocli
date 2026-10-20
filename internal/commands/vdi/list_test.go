package vdi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/testenv"
	"github.com/littlejo/xo-gocli/internal/testutil"
)

const fixtureVDIs = `[
	{
		"id": "11111111-1111-4111-8111-111111111111",
		"uuid": "11111111-1111-4111-8111-111111111111",
		"type": "VDI",
		"name_label": "system disk",
		"size": 10737418240,
		"usage": 5368709120,
		"VDI_type": "system",
		"missing": false,
		"Snapshots": [],
		"tags": [],
		"current_operations": {},
		"other_config": {},
		"$SR": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
		"$VBDs": ["33333333-3333-4333-8333-333333333333"],
		"$poolId": "aaaaaaaa-bbbb-cccc-dddd-000000000009"
	},
	{
		"id": "22222222-2222-4222-8222-222222222222",
		"uuid": "22222222-2222-4222-8222-222222222222",
		"type": "VDI",
		"name_label": "data disk",
		"size": 21474836480,
		"usage": 1073741824,
		"VDI_type": "user",
		"missing": false,
		"Snapshots": [],
		"tags": ["data"],
		"current_operations": {},
		"other_config": {},
		"$SR": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
		"$VBDs": [],
		"$poolId": "aaaaaaaa-bbbb-cccc-dddd-000000000009"
	}
]`

// fakeXO serves the minimal REST surface used by VDI().GetAll.
func fakeXO(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/vdis" {
			http.NotFound(w, r)
			return
		}
		if handler != nil {
			handler(w, r)
			return
		}
		if !testutil.Authorized(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, fixtureVDIs)
	}))
}

func newTestRoot() *cobra.Command {
	return testutil.Root(NewCommand())
}

func runVDI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return testutil.ExecuteCombined(newTestRoot(), args...)
}

func TestVDIListTable(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runVDI(t, "vdi", "list")
	if err != nil {
		t.Fatalf("vdi list: %v", err)
	}
	for _, expected := range []string{"ID", "NAME", "TYPE", "SIZE", "USAGE", "SR",
		"system disk", "data disk", "system", "user", "10.74GB", "21.47GB", "1.074GB"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
	if strings.Count(out, "system disk") != 1 || strings.Count(out, "data disk") != 1 {
		t.Fatalf("unexpected number of rows:\n%s", out)
	}
}

func TestVDIListJSON(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runVDI(t, "vdi", "list", "--output", "json")
	if err != nil {
		t.Fatalf("vdi list --output json: %v", err)
	}

	var vdis []map[string]any
	if err := json.Unmarshal([]byte(out), &vdis); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(vdis) != 2 {
		t.Fatalf("expected 2 VDIs, got %d", len(vdis))
	}
	if vdis[0]["name_label"] != "system disk" || vdis[0]["VDI_type"] != "system" {
		t.Fatalf("unexpected VDI payload: %s", out)
	}
}

func TestVDIListQuery(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runVDI(t, "vdi", "list", "--query", "[?VDI_type==`user`].name_label")
	if err != nil {
		t.Fatalf("vdi list --query: %v", err)
	}
	if !strings.Contains(out, "data disk") || strings.Contains(out, "system disk") {
		t.Fatalf("query should keep only the user VDI:\n%s", out)
	}
}

func TestVDIListTypeFilter(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter") != "VDI_type:user" {
			t.Errorf("expected the VDI_type:user live filter, got %q", r.URL.Query().Get("filter"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	})
	defer server.Close()
	testenv.Isolate(t, server.URL)

	if _, err := runVDI(t, "vdi", "list", "--type", "user"); err != nil {
		t.Fatalf("vdi list --type user: %v", err)
	}
}

func TestVDIListInvalidQuery(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	if _, err := runVDI(t, "vdi", "list", "--query", "[?"); err == nil {
		t.Fatal("expected an error for an invalid query")
	}
}
