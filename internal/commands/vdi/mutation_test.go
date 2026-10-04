package vdi

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
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	getVDIID     = "11111111-1111-4111-8111-111111111111"
	createdVDIID = "44444444-4444-4444-8444-444444444444"
	targetSRID   = "aaaaaaaa-bbbb-cccc-dddd-000000000002"
	migrateTask  = "99999999-1111-4111-8111-999999999999"
)

const fixtureVDI = `{
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
}`

// fixtureTask is the task created by a VDI migration (pending, so the command
// reports it without waiting).
const fixtureTask = `{
	"id": "` + migrateTask + `",
	"status": "pending",
	"start": "2026-09-28T10:05:00.000Z",
	"properties": {"name": "migrate", "type": "VDI"}
}`

// fixtureImage is the content the export endpoint streams and the import
// endpoint records.
const fixtureImage = "RAWDISKIMAGE"

type request struct {
	Method string
	Path   string
	Body   string
}

// mutationServer is a fake XO server for the VDI create/delete endpoints. It
// records every request so tests can assert on method, path and body, and
// answers:
//
//	GET    /rest/v0/vdis/{id} -> fixtureVDI (404 for other ids)
//	POST   /rest/v0/vdis      -> {"id": createdVDIID}
//	DELETE /rest/v0/vdis/{id} -> 200 {}
type mutationServer struct {
	*httptest.Server
	requests []request
}

func newMutationServer(t *testing.T) *mutationServer {
	t.Helper()
	s := &mutationServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := readBody(r)
		s.requests = append(s.requests, request{Method: r.Method, Path: r.URL.Path, Body: body})

		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		// Export: streams the fixture image for the raw and vhd formats.
		// (Matched before the generic /vdis/{id} GET case, which would 404.)
		case r.Method == http.MethodGet && (r.URL.Path == "/rest/v0/vdis/"+getVDIID+".raw" ||
			r.URL.Path == "/rest/v0/vdis/"+getVDIID+".vhd"):
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = fmt.Fprint(w, fixtureImage)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/vdis/"+getVDIID:
			_, _ = fmt.Fprint(w, fixtureVDI)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/vdis/"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/v0/vdis":
			_, _ = fmt.Fprint(w, `{"id": "44444444-4444-4444-8444-444444444444"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/v0/vdis/"+getVDIID:
			_, _ = fmt.Fprint(w, `{}`)
		// Existence check for the migrate destination SR.
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/srs/"+targetSRID:
			_, _ = fmt.Fprint(w, `{"id": "`+targetSRID+`", "name_label": "fast sr"}`)
		// Migrate action: returns the task id.
		case r.Method == http.MethodPost && r.URL.Path == "/rest/v0/vdis/"+getVDIID+"/actions/migrate":
			_, _ = fmt.Fprint(w, `{"taskId": "`+migrateTask+`"}`)
		// The task created by the migrate action (fetched right after).
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/tasks/"+migrateTask:
			_, _ = fmt.Fprint(w, fixtureTask)
		// Tag management.
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/rest/v0/vdis/"+getVDIID+"/tags/"):
			_, _ = fmt.Fprint(w, `{}`)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/rest/v0/vdis/"+getVDIID+"/tags/"):
			_, _ = fmt.Fprint(w, `{}`)
		// Import: accepts the streamed image.
		case r.Method == http.MethodPut && (r.URL.Path == "/rest/v0/vdis/"+getVDIID+".raw" ||
			r.URL.Path == "/rest/v0/vdis/"+getVDIID+".vhd"):
			_, _ = fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func (s *mutationServer) requestByMethod(method string) (request, bool) {
	for _, r := range s.requests {
		if r.Method == method {
			return r, true
		}
	}
	return request{}, false
}

func readBody(r *http.Request) (string, error) {
	data, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	return string(data), err
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

// --- get --------------------------------------------------------------------

// TestVDIGetDetail checks the human view of `vdi get`: it is a detail sheet,
// not the one-row table shared with `vdi list`. The mutation server does not
// serve SRs, so the SR falls back to its raw id; it also does not serve VBDs,
// so the "Attached to" line is simply omitted (best effort).
func TestVDIGetDetail(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "vdi", "get", getVDIID)
	if err != nil {
		t.Fatalf("vdi get: %v", err)
	}
	for _, expected := range []string{
		"VDI system disk",
		output.DetailField("SR", "aaaaaaaa-bbbb-cccc-dddd-000000000001"),
		output.DetailField("Type", "system"),
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("get output missing %q:\n%s", expected, out)
		}
	}
}

func TestVDIGetJSON(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runGet(t, "vdi", "get", getVDIID, "--output", "json")
	if err != nil {
		t.Fatalf("vdi get --output json: %v", err)
	}
	var vdi map[string]any
	if err := json.Unmarshal([]byte(out), &vdi); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if vdi["name_label"] != "system disk" {
		t.Fatalf("unexpected VDI payload: %s", out)
	}
}

func TestVDIGetNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	_, err := runGet(t, "vdi", "get", missing)
	if err == nil {
		t.Fatal("expected an error when the VDI does not exist")
	}
	if !strings.Contains(err.Error(), `VDI "99999999-9999-4999-8999-999999999999" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVDIGetBadID(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runGet(t, "vdi", "get", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
}

// --- create -----------------------------------------------------------------

func TestVDICreate(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVDI(t, "vdi", "create", "data-01", "--sr", "aaaaaaaa-bbbb-cccc-dddd-000000000001", "--size", "10G", "--description", "data disk")
	if err != nil {
		t.Fatalf("vdi create: %v", err)
	}
	if !strings.Contains(out, "created") || !strings.Contains(out, createdVDIID) {
		t.Fatalf("create output should report the created VDI id:\n%s", out)
	}
	// The next-step hint should point at the attach command.
	if !strings.Contains(out, "xo vbd create") {
		t.Fatalf("create output should hint the attach command:\n%s", out)
	}

	req, ok := server.requestByMethod(http.MethodPost)
	if !ok {
		t.Fatal("expected a POST create request")
	}
	if req.Path != "/rest/v0/vdis" {
		t.Fatalf("unexpected create path: %s", req.Path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse create body: %v\n%s", err, req.Body)
	}
	if body["srId"] != "aaaaaaaa-bbbb-cccc-dddd-000000000001" {
		t.Fatalf("expected srId, got %s", req.Body)
	}
	// 10G must be converted to bytes.
	if got, _ := body["virtual_size"].(float64); got != 10*1024*1024*1024 {
		t.Fatalf("expected virtual_size=10737418240 (bytes), got %v", body["virtual_size"])
	}
	if body["name_label"] != "data-01" {
		t.Fatalf("expected name_label=data-01, got %s", req.Body)
	}
	if body["name_description"] != "data disk" {
		t.Fatalf("expected name_description, got %s", req.Body)
	}
	if _, present := body["sharable"]; present {
		t.Fatalf("sharable must be omitted when --shared is not given: %s", req.Body)
	}
}

func TestVDICreateRequiresSRAndSize(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runVDI(t, "vdi", "create", "data-01", "--size", "10G"); err == nil {
		t.Fatal("expected an error when --sr is missing")
	}
	if _, err := runVDI(t, "vdi", "create", "data-01", "--sr", "aaaaaaaa-bbbb-cccc-dddd-000000000001"); err == nil {
		t.Fatal("expected an error when --size is missing")
	}
	if _, err := runVDI(t, "vdi", "create", "data-01", "--sr", "aaaaaaaa-bbbb-cccc-dddd-000000000001", "--size", "not-a-size"); err == nil {
		t.Fatal("expected an error for an invalid --size value")
	}
	if _, ok := server.requestByMethod(http.MethodPost); ok {
		t.Fatal("create must not be executed with missing/invalid flags")
	}
}

func TestVDICreateSharedAndTags(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runVDI(t, "vdi", "create", "shared", "--sr", "aaaaaaaa-bbbb-cccc-dddd-000000000001", "--size", "1G", "--shared", "--tags", "common,scratch"); err != nil {
		t.Fatalf("vdi create --shared: %v", err)
	}
	req, _ := server.requestByMethod(http.MethodPost)
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse create body: %v\n%s", err, req.Body)
	}
	if got, _ := body["sharable"].(bool); !got {
		t.Fatalf("expected sharable=true, got %s", req.Body)
	}
	tags, ok := body["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "common" || tags[1] != "scratch" {
		t.Fatalf("expected tags=[common scratch], got %s", req.Body)
	}
}

// --- delete -----------------------------------------------------------------

func TestVDIDelete(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVDI(t, "vdi", "delete", getVDIID, "--yes")
	if err != nil {
		t.Fatalf("vdi delete: %v", err)
	}
	if !strings.Contains(out, "system disk") {
		t.Fatalf("delete output should mention the VDI name:\n%s", out)
	}
	if _, ok := server.requestByMethod(http.MethodDelete); !ok {
		t.Fatal("expected a DELETE request")
	}
}

func TestVDIDeleteJSON(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runVDI(t, "vdi", "delete", getVDIID, "--yes", "--output", "json")
	if err != nil {
		t.Fatalf("vdi delete --output json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("delete JSON output is not valid JSON: %v\n%s", err, out)
	}
	if doc["vdi"] != "system disk" {
		t.Fatalf("unexpected delete JSON payload: %s", out)
	}
}

func TestVDIDeleteRequiresConfirmation(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runVDI(t, "vdi", "delete", getVDIID)
	if err == nil {
		t.Fatal("expected an error when no confirmation is given and stdin is not a terminal")
	}
	if _, ok := server.requestByMethod(http.MethodDelete); ok {
		t.Fatal("the VDI must not be deleted without confirmation")
	}
}

func TestVDIDeleteSkipsConfirmationWithEnvYes(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)
	t.Setenv("XOA_YES", "1")

	if _, err := runVDI(t, "vdi", "delete", getVDIID); err != nil {
		t.Fatalf("vdi delete with XOA_YES=1: %v", err)
	}
	if _, ok := server.requestByMethod(http.MethodDelete); !ok {
		t.Fatal("expected a DELETE request when XOA_YES=1")
	}
}

func TestVDIDeleteNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	_, err := runVDI(t, "vdi", "delete", missing, "--yes")
	if err == nil {
		t.Fatal("expected an error when the VDI does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := server.requestByMethod(http.MethodDelete); ok {
		t.Fatal("the VDI must not be deleted when it does not exist")
	}
}
