package vbd

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

const fixtureVBDs = `[
	{
		"id": "33333333-3333-4333-8333-333333333333",
		"uuid": "33333333-3333-4333-8333-333333333333",
		"type": "VBD",
		"$pool": "aaaaaaaa-bbbb-cccc-dddd-000000000009",
		"_xapiRef": "VBD:111",
		"attached": true,
		"bootable": false,
		"device": "xvda",
		"is_cd_drive": false,
		"position": "0",
		"read_only": false,
		"VDI": "11111111-1111-4111-8111-111111111111",
		"VM": "550e8400-e29b-41d4-a716-446655440001"
	},
	{
		"id": "44444444-4444-4444-8444-444444444444",
		"uuid": "44444444-4444-4444-8444-444444444444",
		"type": "VBD",
		"$pool": "aaaaaaaa-bbbb-cccc-dddd-000000000009",
		"_xapiRef": "VBD:222",
		"attached": false,
		"bootable": true,
		"device": null,
		"is_cd_drive": false,
		"position": "1",
		"read_only": true,
		"VDI": "22222222-2222-4222-8222-222222222222",
		"VM": "550e8400-e29b-41d4-a716-446655440001"
	}
]`

// fakeXO serves the minimal REST surface used by VBD().GetAll.
func fakeXO(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/vbds" {
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
		_, _ = fmt.Fprint(w, fixtureVBDs)
	}))
}

// isolatePointers points the CLI at the fake server and at a throwaway config
// file, and clears any ambient XOA_ variables.
func isolatePointers(t *testing.T, url string) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE", "XOA_YES", "XOA_WAIT"} {
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
	root.AddCommand(NewCommand())
	return root
}

func runVBD(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestVBDListTable(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "list")
	if err != nil {
		t.Fatalf("vbd list: %v", err)
	}
	for _, expected := range []string{"ID", "VM", "VDI", "DEVICE", "MODE", "ATTACHED",
		"33333333-3333-4333-8333-333333333333", "xvda", "RW", "yes", "RO", "no"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
}

// Regression: the REST API exposes the XAPI "userdevice" under the "position"
// key as a string whose content is data dependent — a numeric index ("0") on
// some stacks or a device name ("xvdb") on others. A device name must not fail
// the unmarshal of the whole VBD listing.
func TestVBDListDeviceNamePosition(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[
			{
				"id": "33333333-3333-4333-8333-333333333333",
				"uuid": "33333333-3333-4333-8333-333333333333",
				"type": "VBD",
				"$pool": "aaaaaaaa-bbbb-cccc-dddd-000000000009",
				"attached": true,
				"bootable": false,
				"device": "xvdb",
				"is_cd_drive": false,
				"position": "xvdb",
				"read_only": false,
				"VDI": "11111111-1111-4111-8111-111111111111",
				"VM": "550e8400-e29b-41d4-a716-446655440001"
			}
		]`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "list")
	if err != nil {
		t.Fatalf("vbd list with a device-name position must not fail unmarshal: %v", err)
	}
	if !strings.Contains(out, "33333333-3333-4333-8333-333333333333") {
		t.Errorf("expected the VBD id in the listing:\n%s", out)
	}
}

func TestVBDListJSON(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "list", "--output", "json")
	if err != nil {
		t.Fatalf("vbd list --output json: %v", err)
	}

	var vbds []map[string]any
	if err := json.Unmarshal([]byte(out), &vbds); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(vbds) != 2 {
		t.Fatalf("expected 2 VBDs, got %d", len(vbds))
	}
	if vbds[0]["device"] != "xvda" || vbds[0]["attached"] != true {
		t.Fatalf("unexpected VBD payload: %s", out)
	}
}

func TestVBDListByVM(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter") != "VM:550e8400-e29b-41d4-a716-446655440001" {
			t.Errorf("expected the VM live filter, got %q", r.URL.Query().Get("filter"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[]`)
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runVBD(t, "vbd", "list", "--vm", "550e8400-e29b-41d4-a716-446655440001"); err != nil {
		t.Fatalf("vbd list --vm: %v", err)
	}
}

func TestVBDListBadVM(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runVBD(t, "vbd", "list", "--vm", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid --vm id")
	}
}

func TestVBDListQuery(t *testing.T) {
	server := fakeXO(t, nil)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVBD(t, "vbd", "list", "--query", "[?attached].device")
	if err != nil {
		t.Fatalf("vbd list --query: %v", err)
	}
	if !strings.Contains(out, "xvda") {
		t.Fatalf("query should keep only the attached VBD:\n%s", out)
	}
}
