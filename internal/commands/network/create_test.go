package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
)

const (
	// createPoolID is the pool the create actions target.
	createPoolID = "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	// createPifID is a valid PIF UUID for the create tests.
	createPifID = "aaaaaaaa-bbbb-cccc-dddd-000000000002"
	// createdNetworkID is the id the create task reports as the new network.
	createdNetworkID = "22222222-2222-4222-8222-222222222222"
)

// createdNetworkTask is the success task returned by the create_* pool
// actions: it carries the created network id in its result.
const createdNetworkTask = `{
	"id": "task-123",
	"status": "success",
	"start": 1700000000000,
	"end": 1700000001000,
	"result": {"id": "22222222-2222-4222-8222-222222222222"}
}`

// createdNetworkFixture is the network served on the re-fetch after creation.
const createdNetworkFixture = `{
	"id": "22222222-2222-4222-8222-222222222222",
	"uuid": "22222222-2222-4222-8222-222222222222",
	"type": "network",
	"name_label": "web",
	"MTU": 9000,
	"bridge": "xenbr1",
	"automatic": false,
	"defaultIsLocked": false,
	"isBonded": false,
	"VIFs": [],
	"PIFs": [],
	"$pool": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
}`

// createServer is a fake XO server for the network create endpoints. It
// records the POST request (method, path, body) so tests can assert on the
// pool action and its parameters, and answers:
//
//	POST /rest/v0/pools/{id}/actions/create_network           -> {"taskId":"task-123"}
//	POST /rest/v0/pools/{id}/actions/create_internal_network  -> {"taskId":"task-123"}
//	POST /rest/v0/pools/{id}/actions/create_bonded_network    -> {"taskId":"task-123"}
//	GET  /rest/v0/tasks/{id}                                  -> createdNetworkTask
//	GET  /rest/v0/networks/{createdNetworkID}                 -> createdNetworkFixture
type createServer struct {
	*httptest.Server
	postPath string
	postBody string
}

func newCreateServer(t *testing.T) *createServer {
	t.Helper()
	s := &createServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/actions/create_"):
			body, _ := readBody(r)
			s.postPath = r.URL.Path
			s.postBody = body
			_, _ = fmt.Fprint(w, `{"taskId":"task-123"}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/tasks/"):
			_, _ = fmt.Fprint(w, createdNetworkTask)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/networks/"+createdNetworkID:
			_, _ = fmt.Fprint(w, createdNetworkFixture)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func TestNetworkCreate(t *testing.T) {
	server := newCreateServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runNetwork(t, "create", "web", "--pool", createPoolID, "--pif", createPifID, "--vlan", "100", "--mtu", "9000", "--nbd")
	if err != nil {
		t.Fatalf("network create: %v", err)
	}
	// The re-fetch returns the created fixture; assert on the created marker
	// and the re-fetched id, not on the requested MTU (server default wins).
	if !strings.Contains(out, "created") || !strings.Contains(out, createdNetworkID) {
		t.Fatalf("create output should report the created network and its id:\n%s", out)
	}

	if server.postPath != "/rest/v0/pools/"+createPoolID+"/actions/create_network" {
		t.Fatalf("unexpected create path: %s", server.postPath)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(server.postBody), &body); err != nil {
		t.Fatalf("cannot parse create body: %v\n%s", err, server.postBody)
	}
	if body["name"] != "web" {
		t.Fatalf("expected name=web, got %s", server.postBody)
	}
	if body["pif"] != createPifID {
		t.Fatalf("expected pif=%s, got %s", createPifID, server.postBody)
	}
	if got, _ := body["vlan"].(float64); got != 100 {
		t.Fatalf("expected vlan=100, got %v", body["vlan"])
	}
	if got, _ := body["mtu"].(float64); got != 9000 {
		t.Fatalf("expected mtu=9000, got %v", body["mtu"])
	}
	if got, _ := body["nbd"].(bool); !got {
		t.Fatalf("expected nbd=true, got %s", server.postBody)
	}
}

func TestNetworkCreateOmitsOptionalFields(t *testing.T) {
	server := newCreateServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runNetwork(t, "create", "web", "--pool", createPoolID, "--pif", createPifID); err != nil {
		t.Fatalf("network create: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(server.postBody), &body); err != nil {
		t.Fatalf("cannot parse create body: %v\n%s", err, server.postBody)
	}
	// With no --mtu / --nbd / --description the optional fields must be
	// omitted so the server defaults apply; vlan defaults to 0.
	if _, present := body["mtu"]; present {
		t.Fatalf("mtu must be omitted when --mtu is not given: %s", server.postBody)
	}
	if _, present := body["nbd"]; present {
		t.Fatalf("nbd must be omitted when --nbd is not given: %s", server.postBody)
	}
	if _, present := body["description"]; present {
		t.Fatalf("description must be omitted when --description is not given: %s", server.postBody)
	}
	if got, _ := body["vlan"].(float64); got != 0 {
		t.Fatalf("expected default vlan=0, got %v", body["vlan"])
	}
}

func TestNetworkCreateRequiresPoolAndPif(t *testing.T) {
	server := newCreateServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runNetwork(t, "create", "web"); err == nil {
		t.Fatal("expected an error when --pool and --pif are missing")
	}
	if _, err := runNetwork(t, "create", "web", "--pool", createPoolID); err == nil {
		t.Fatal("expected an error when --pif is missing")
	}
	if _, err := runNetwork(t, "create", "web", "--pif", createPifID); err == nil {
		t.Fatal("expected an error when --pool is missing")
	}
	if server.postPath != "" {
		t.Fatalf("create action must not be executed with missing --pool/--pif: %s", server.postPath)
	}
}

func TestNetworkCreateJSON(t *testing.T) {
	server := newCreateServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runNetwork(t, "create", "web", "--pool", createPoolID, "--pif", createPifID, "--output", "json")
	if err != nil {
		t.Fatalf("network create --output json: %v", err)
	}
	// Machine output must be the re-fetched network as valid JSON.
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("create JSON output is not valid JSON: %v\n%s", err, out)
	}
	if doc["id"] != createdNetworkID {
		t.Fatalf("unexpected create JSON payload: %s", out)
	}
}

func TestNetworkCreateInternal(t *testing.T) {
	server := newCreateServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runNetwork(t, "create-internal", "internal", "--pool", createPoolID, "--description", "vm-to-vm")
	if err != nil {
		t.Fatalf("network create-internal: %v", err)
	}
	if !strings.Contains(out, "created") || !strings.Contains(out, createdNetworkID) {
		t.Fatalf("create-internal output should report the created network:\n%s", out)
	}
	if server.postPath != "/rest/v0/pools/"+createPoolID+"/actions/create_internal_network" {
		t.Fatalf("unexpected create-internal path: %s", server.postPath)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(server.postBody), &body); err != nil {
		t.Fatalf("cannot parse create-internal body: %v\n%s", err, server.postBody)
	}
	if body["name"] != "internal" || body["description"] != "vm-to-vm" {
		t.Fatalf("unexpected create-internal body: %s", server.postBody)
	}
	if _, present := body["pif"]; present {
		t.Fatalf("an internal network must not carry a pif: %s", server.postBody)
	}
}

func TestNetworkCreateInternalRequiresPool(t *testing.T) {
	server := newCreateServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runNetwork(t, "create-internal", "internal"); err == nil {
		t.Fatal("expected an error when --pool is missing")
	}
	if server.postPath != "" {
		t.Fatalf("create-internal must not be executed without --pool: %s", server.postPath)
	}
}

func TestNetworkCreateBonded(t *testing.T) {
	server := newCreateServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	pif2 := "aaaaaaaa-bbbb-cccc-dddd-000000000003"
	out, err := runNetwork(t, "create-bonded", "bond0", "--pool", createPoolID,
		"--pifs", createPifID+","+pif2, "--bond-mode", "active-backup")
	if err != nil {
		t.Fatalf("network create-bonded: %v", err)
	}
	if !strings.Contains(out, "created") || !strings.Contains(out, createdNetworkID) {
		t.Fatalf("create-bonded output should report the created network:\n%s", out)
	}
	if server.postPath != "/rest/v0/pools/"+createPoolID+"/actions/create_bonded_network" {
		t.Fatalf("unexpected create-bonded path: %s", server.postPath)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(server.postBody), &body); err != nil {
		t.Fatalf("cannot parse create-bonded body: %v\n%s", err, server.postBody)
	}
	if body["bondMode"] != "active-backup" {
		t.Fatalf("expected bondMode=active-backup, got %s", server.postBody)
	}
	pifIDs, ok := body["pifIds"].([]any)
	if !ok || len(pifIDs) != 2 || pifIDs[0] != createPifID || pifIDs[1] != pif2 {
		t.Fatalf("expected two pifIds, got %s", server.postBody)
	}
}

func TestNetworkCreateBondedValidatesMode(t *testing.T) {
	server := newCreateServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	// Missing mode.
	if _, err := runNetwork(t, "create-bonded", "bond0", "--pool", createPoolID, "--pifs", createPifID); err == nil {
		t.Fatal("expected an error when --bond-mode is missing")
	}
	// Unknown mode.
	if _, err := runNetwork(t, "create-bonded", "bond0", "--pool", createPoolID, "--pifs", createPifID, "--bond-mode", "bogus"); err == nil {
		t.Fatal("expected an error for an unknown --bond-mode")
	}
	// Empty pifs list.
	if _, err := runNetwork(t, "create-bonded", "bond0", "--pool", createPoolID, "--pifs", " , ", "--bond-mode", "lacp"); err == nil {
		t.Fatal("expected an error for an empty --pifs list")
	}
	if server.postPath != "" {
		t.Fatalf("create-bonded must not be executed with invalid flags: %s", server.postPath)
	}
}

// readBody reads and closes the request body, returning it as a string.
func readBody(r *http.Request) (string, error) {
	data, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	return string(data), err
}

// newNetworkTestRoot builds the test root 'xo' command with the full network
// command group registered.
func newNetworkTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	root.AddCommand(NewCommand())
	return root
}

// runNetwork executes the 'xo network <args...>' command against the fake
// server and returns the combined stdout/stderr output.
func runNetwork(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newNetworkTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"network"}, args...))
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}
