package vm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const createdVMTask = `{
	"id": "task-123",
	"status": "success",
	"start": 1700000000000,
	"end": 1700000001000,
	"result": {"id": "550e8400-e29b-41d4-a716-446655440002"}
}`

// mutationServer is a fake XO server for the VM create/tag/update endpoints. It
// records every request so tests can assert on method, path and body, and
// answers:
//
//	POST /rest/v0/pools/{id}/actions/create_vm -> {"taskId":"task-123"}
//	GET  /rest/v0/tasks/{id}                    -> createdVMTask (success, with result.id)
//	GET  /rest/v0/vms/{id}                      -> fixtureVM
//	PUT    /rest/v0/vms/{id}/tags/{tag}         -> 200 {}
//	DELETE /rest/v0/vms/{id}/tags/{tag}         -> 200 {}
//	PATCH  /rest/v0/vms/{id}                    -> 200 {}
type mutationServer struct {
	*httptest.Server
	t        *testing.T
	requests []request
	// vmResponse overrides the GET /rest/v0/vms/{id} answer (the create
	// re-fetch); fixtureVM is used when empty.
	vmResponse string
	// vm404 makes GET /rest/v0/vms/{id} answer 404 (for the wait not-found
	// path).
	vm404 bool
}

func newMutationServer(t *testing.T) *mutationServer {
	t.Helper()
	s := &mutationServer{t: t}
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
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/actions/create_vm"):
			_, _ = fmt.Fprint(w, `{"taskId":"task-123"}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/tasks/"):
			_, _ = fmt.Fprint(w, createdVMTask)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			if s.vm404 {
				http.NotFound(w, r)
				return
			}
			if s.vmResponse != "" {
				_, _ = fmt.Fprint(w, s.vmResponse)
				return
			}
			_, _ = fmt.Fprint(w, fixtureVM)
		case (r.Method == http.MethodPut || r.Method == http.MethodDelete) && strings.Contains(r.URL.Path, "/tags/"):
			_, _ = fmt.Fprint(w, `{}`)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			_, _ = fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

// requestByMethod returns the first recorded request matching method, or
// request{} and false.
func (s *mutationServer) requestByMethod(method string) (request, bool) {
	for _, r := range s.requests {
		if r.Method == method {
			return r, true
		}
	}
	return request{}, false
}

// --- create -----------------------------------------------------------------

func TestVMCreate(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	template := "aaaaaaaa-bbbb-cccc-dddd-000000000009"

	out, err := runVM(t, "vm", "create", "web-02", "--pool", pool, "--template", template, "--memory", "4G")
	if err != nil {
		t.Fatalf("vm create: %v", err)
	}
	// The create flow is: POST create_vm -> wait on the task -> re-fetch the VM.
	// The re-fetch returns the fixed fixture in the fake, so assert on the
	// success marker and the re-fetched id, not on the requested name.
	if !strings.Contains(out, "created") {
		t.Fatalf("create output should report the VM was created:\n%s", out)
	}
	if !strings.Contains(out, "550e8400-e29b-41d4-a716-446655440001") {
		t.Fatalf("create output should include the re-fetched VM id:\n%s", out)
	}

	// The create_vm action must target the pool.
	req, ok := server.requestByMethod(http.MethodPost)
	if !ok {
		t.Fatal("expected a POST create_vm request")
	}
	wantPath := "/rest/v0/pools/" + pool + "/actions/create_vm"
	if req.Path != wantPath {
		t.Fatalf("unexpected create path: %s", req.Path)
	}

	// The body must carry the name and the template (template is a required
	// field of create_vm).
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse create body: %v\n%s", err, req.Body)
	}
	if body["name_label"] != "web-02" {
		t.Fatalf("expected name_label=web-02, got %s", req.Body)
	}
	if body["template"] != template {
		t.Fatalf("expected template=%s, got %s", template, body["template"])
	}
	// 4G must be converted to bytes.
	if got, _ := body["memory"].(float64); got != 4*1024*1024*1024 {
		t.Fatalf("expected memory=4294967296 (bytes), got %v", body["memory"])
	}
}

// TestVMCreateStartHint checks the "Start it with" hint in the create output
// (C4): the hint must only be shown when the created VM is actually halted.
// A VM created with --boot is already starting — the re-fetch reports the
// start operation in current_operations while the raw power_state still
// lags at Halted — so a hint there would be noise and is omitted.
func TestVMCreateStartHint(t *testing.T) {
	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	template := "aaaaaaaa-bbbb-cccc-dddd-000000000009"
	id := "550e8400-e29b-41d4-a716-446655440001"
	startHint := "Start it with: xo vm start " + id

	t.Run("halted VM suggests start", func(t *testing.T) {
		server := newMutationServer(t)
		defer server.Close()
		isolateVM(t, server.URL)

		// Default re-fetch: fixtureVM, halted, no operation in flight.
		out, err := runVM(t, "vm", "create", "web-02", "--pool", pool, "--template", template)
		if err != nil {
			t.Fatalf("vm create: %v\n%s", err, out)
		}
		if !strings.Contains(out, "state:  Halted") {
			t.Fatalf("expected the halted state:\n%s", out)
		}
		if !strings.Contains(out, startHint) {
			t.Fatalf("a halted VM should be told how to start it:\n%s", out)
		}
	})

	t.Run("booted VM omits the start hint", func(t *testing.T) {
		server := newMutationServer(t)
		defer server.Close()
		isolateVM(t, server.URL)

		// Re-fetch mid-boot: the raw power_state still lags at Halted, but
		// the start operation is in flight.
		server.vmResponse = `{
			"id": "` + id + `",
			"uuid": "` + id + `",
			"type": "vm",
			"name_label": "web-02",
			"power_state": "Halted",
			"memory": {"size": 2147483648},
			"CPUs": {"number": 2},
			"current_operations": {"00000000-0000-0000-0000-0000000000aa": "start"},
			"$container": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
		}`

		out, err := runVM(t, "vm", "create", "web-02", "--pool", pool, "--template", template, "--boot")
		if err != nil {
			t.Fatalf("vm create --boot: %v\n%s", err, out)
		}
		if !strings.Contains(out, "state:  Starting") {
			t.Fatalf("expected the starting state:\n%s", out)
		}
		if strings.Contains(out, startHint) {
			t.Fatalf("a starting VM must not be told to start it:\n%s", out)
		}
	})
}

func TestVMCreateRequiresPoolAndTemplate(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"

	if _, err := runVM(t, "vm", "create", "web-02", "--pool", pool); err == nil {
		t.Fatal("expected an error when --template is missing")
	}
	if _, err := runVM(t, "vm", "create", "web-02", "--template", pool); err == nil {
		t.Fatal("expected an error when --pool is missing")
	}
	// No create_vm request must have been sent.
	if _, ok := server.requestByMethod(http.MethodPost); ok {
		t.Fatal("create_vm must not be executed with missing --pool/--template")
	}
}

func TestVMCreateBadMemory(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	template := "aaaaaaaa-bbbb-cccc-dddd-000000000009"

	if _, err := runVM(t, "vm", "create", "web-02", "--pool", pool, "--template", template, "--memory", "not-a-size"); err == nil {
		t.Fatal("expected an error for an invalid --memory value")
	}
}

func TestVMCreateCompositeTemplateID(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	// The composite id exactly as 'xo template list' prints it:
	// <poolId>-<templateUuid> (36 + 1 + 36 chars).
	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	template := "aaaaaaaa-bbbb-cccc-dddd-000000000009"
	composite := pool + "-" + template

	out, err := runVM(t, "vm", "create", "web-02", "--pool", pool, "--template", composite)
	if err != nil {
		t.Fatalf("vm create with composite --template: %v", err)
	}
	if !strings.Contains(out, "created") {
		t.Fatalf("create output should report the VM was created:\n%s", out)
	}

	// The create_vm body must carry the BARE template uuid (create_vm wants
	// XoVmTemplate['uuid']), not the composite id.
	req, ok := server.requestByMethod(http.MethodPost)
	if !ok {
		t.Fatal("expected a POST create_vm request")
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse create body: %v\n%s", err, req.Body)
	}
	if body["template"] != template {
		t.Fatalf("expected the bare template uuid %s in the body, got %s", template, body["template"])
	}
}

func TestVMCreateInvalidTemplateID(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	// Same 73-char shape as a composite id, but the trailing segment is not a
	// UUID (and the leading one is not either): it must be rejected, not
	// silently truncated.
	bad := "aaaaaaaa-bbbb-cccc-dddd-000000000009-6959dfe8-534c-4c58-8a8c-3c379229354z"
	if _, err := runVM(t, "vm", "create", "web-02", "--pool", pool, "--template", bad); err == nil {
		t.Fatalf("expected an error for the malformed composite id %q", bad)
	}
	if _, ok := server.requestByMethod(http.MethodPost); ok {
		t.Fatal("create_vm must not be executed with an invalid --template")
	}
}

// --- tag --------------------------------------------------------------------

func TestVMTagAdd(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	out, err := runVM(t, "vm", "tag", "add", "550e8400-e29b-41d4-a716-446655440001", "production")
	if err != nil {
		t.Fatalf("vm tag add: %v", err)
	}
	if !strings.Contains(out, "production") {
		t.Fatalf("tag add output should mention the tag:\n%s", out)
	}

	req, ok := server.requestByMethod(http.MethodPut)
	if !ok {
		t.Fatal("expected a PUT tag request")
	}
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/tags/production" {
		t.Fatalf("unexpected tag path: %s", req.Path)
	}
}

func TestVMTagRemove(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "tag", "remove", "550e8400-e29b-41d4-a716-446655440001", "production"); err != nil {
		t.Fatalf("vm tag remove: %v", err)
	}
	req, ok := server.requestByMethod(http.MethodDelete)
	if !ok {
		t.Fatal("expected a DELETE tag request")
	}
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001/tags/production" {
		t.Fatalf("unexpected tag path: %s", req.Path)
	}
}

// --- update -----------------------------------------------------------------

func TestVMUpdateName(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "update", "550e8400-e29b-41d4-a716-446655440001", "--name", "web-renamed"); err != nil {
		t.Fatalf("vm update --name: %v", err)
	}
	req, ok := server.requestByMethod(http.MethodPatch)
	if !ok {
		t.Fatal("expected a PATCH request")
	}
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001" {
		t.Fatalf("unexpected update path: %s", req.Path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse update body: %v\n%s", err, req.Body)
	}
	if body["nameLabel"] != "web-renamed" {
		t.Fatalf("expected nameLabel=web-renamed, got %s", req.Body)
	}
	if _, present := body["nameDescription"]; present {
		t.Fatalf("nameDescription must not be sent when --description is not given: %s", req.Body)
	}
}

func TestVMUpdateTags(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "update", "550e8400-e29b-41d4-a716-446655440001", "--tags", "production,web"); err != nil {
		t.Fatalf("vm update --tags: %v", err)
	}
	req, ok := server.requestByMethod(http.MethodPatch)
	if !ok {
		t.Fatal("expected a PATCH request")
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse update body: %v\n%s", err, req.Body)
	}
	tags, ok := body["tags"].([]any)
	if !ok {
		t.Fatalf("expected tags to be an array, got %T: %s", body["tags"], req.Body)
	}
	if len(tags) != 2 || tags[0] != "production" || tags[1] != "web" {
		t.Fatalf("expected tags=[production web], got %s", req.Body)
	}
	if _, present := body["nameLabel"]; present {
		t.Fatalf("nameLabel must not be sent when --name is not given: %s", req.Body)
	}
}

func TestVMUpdateBadTags(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "update", "550e8400-e29b-41d4-a716-446655440001", "--tags", " , "); err == nil {
		t.Fatal("expected an error for an empty --tags list")
	}
	if _, ok := server.requestByMethod(http.MethodPatch); ok {
		t.Fatal("update must not be executed for an empty --tags list")
	}
}

func TestVMUpdateRequiresField(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "update", "550e8400-e29b-41d4-a716-446655440001"); err == nil {
		t.Fatal("expected an error when neither --name nor --description is given")
	}
	if _, ok := server.requestByMethod(http.MethodPatch); ok {
		t.Fatal("update must not be executed with nothing to change")
	}
}

// --- parseMemory unit tests --------------------------------------------------

func TestParseMemory(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"2147483648", 2147483648},
		{"2G", 2 << 30},
		{"2GiB", 2 << 30},
		{"512M", 512 << 20},
		{"1MiB", 1 << 20},
		{"1024K", 1024 << 10},
		{" 1G ", 1 << 30},
	}
	for _, c := range cases {
		got, err := parseMemory(c.in)
		if err != nil {
			t.Fatalf("parseMemory(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("parseMemory(%q) = %d, want %d", c.in, got, c.want)
		}
	}

	for _, bad := range []string{"", "0", "-1", "abc", "1.5G"} {
		if _, err := parseMemory(bad); err == nil {
			t.Fatalf("parseMemory(%q) should fail", bad)
		}
	}
}

func TestParseTags(t *testing.T) {
	got, err := parseTags("production, web, ci")
	if err != nil {
		t.Fatalf("parseTags: %v", err)
	}
	want := []string{"production", "web", "ci"}
	if len(got) != len(want) {
		t.Fatalf("parseTags = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseTags = %v, want %v", got, want)
		}
	}

	if tags, err := parseTags(""); err != nil || tags != nil {
		t.Fatalf("parseTags(\"\") = %v, %v; want nil, nil", tags, err)
	}
	if _, err := parseTags(" , "); err == nil {
		t.Fatal("parseTags(\" , \") should fail")
	}
}
