package vm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/testutil"
)

const fixturePool = `{
	"id": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
	"uuid": "aaaaaaaa-bbbb-cccc-dddd-000000000001",
	"name_label": "pool-01",
	"state": "Running"
}`

// xvaServer is a fake XO server for the VM XVA/OVA export and import
// endpoints. It records every request so tests can assert on method, path and
// query, and answers:
//
//	GET  /rest/v0/vms/{id}            -> fixtureVM
//	GET  /rest/v0/pools/{id}          -> fixturePool
//	GET  /rest/v0/vms/{id}.xva        -> "XVA-ARCHIVE"
//	GET  /rest/v0/vms/{id}.ova        -> "OVA-ARCHIVE"
//	POST /rest/v0/pools/{id}/vms      -> {"id": <new VM id>}
type xvaServer struct {
	*httptest.Server
	requests []request
}

func newXVAServer(t *testing.T) *xvaServer {
	t.Helper()
	s := &xvaServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests = append(s.requests, request{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})

		if !testutil.Authorized(w, r) {
			return
		}

		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, ".xva"):
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = fmt.Fprint(w, "XVA-ARCHIVE")
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, ".ova"):
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = fmt.Fprint(w, "OVA-ARCHIVE")
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/pools/"):
			_, _ = fmt.Fprint(w, fixturePool)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
			_, _ = fmt.Fprint(w, fixtureVM)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/pools/") && strings.HasSuffix(r.URL.Path, "/vms"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"id":"9fe12ca3-d75d-cfb0-492e-cfd2bc6c568f"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func (s *xvaServer) requestByMethod(method string) (request, bool) {
	for _, r := range s.requests {
		if r.Method == method {
			return r, true
		}
	}
	return request{}, false
}

// xvaRequest returns the export download request (the one whose path carries
// the archive extension). It is distinct from the initial existence-check
// GET /vms/{id} that precedes it.
func (s *xvaServer) xvaRequest() (request, bool) {
	for _, r := range s.requests {
		if r.Method == http.MethodGet && (strings.Contains(r.Path, ".xva") || strings.Contains(r.Path, ".ova")) {
			return r, true
		}
	}
	return request{}, false
}

// --- export -----------------------------------------------------------------

func TestVMExportToFile(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	dir := t.TempDir()
	dst := dir + "/web-01.xva"
	out, err := runVM(t, "vm", "export", "550e8400-e29b-41d4-a716-446655440001", "--file", dst)
	if err != nil {
		t.Fatalf("vm export: %v", err)
	}
	if !strings.Contains(out, "web-01") || !strings.Contains(out, dst) {
		t.Fatalf("export output should mention the VM and the destination:\n%s", out)
	}
	// The archive content must have been written to the file.
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("cannot read the exported file: %v", err)
	}
	if string(data) != "XVA-ARCHIVE" {
		t.Fatalf("unexpected exported content: %q", data)
	}

	req, ok := server.xvaRequest()
	if !ok {
		t.Fatal("expected a GET .xva export request")
	}
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001.xva" {
		t.Fatalf("unexpected export path: %s", req.Path)
	}
	// Compress defaults to true for XVA.
	if req.Query != "compress=true" {
		t.Fatalf("expected compress=true in the query, got %q", req.Query)
	}
}

func TestVMExportUncompressed(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "export", "550e8400-e29b-41d4-a716-446655440001", "--file", t.TempDir()+"/x.xva", "--compress=false"); err != nil {
		t.Fatalf("vm export --compress=false: %v", err)
	}
	req, _ := server.xvaRequest()
	if req.Query != "compress=false" {
		t.Fatalf("expected compress=false in the query, got %q", req.Query)
	}
}

func TestVMExportOVA(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "export", "550e8400-e29b-41d4-a716-446655440001", "--file", t.TempDir()+"/x.ova", "--format", "ova"); err != nil {
		t.Fatalf("vm export --format ova: %v", err)
	}
	req, _ := server.xvaRequest()
	if req.Path != "/rest/v0/vms/550e8400-e29b-41d4-a716-446655440001.ova" {
		t.Fatalf("unexpected export path: %s", req.Path)
	}
	// compress is an XVA-only parameter: it must not be sent for OVA.
	if req.Query != "" {
		t.Fatalf("expected no query for OVA export, got %q", req.Query)
	}
}

func TestVMExportBadFormat(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	if _, err := runVM(t, "vm", "export", "550e8400-e29b-41d4-a716-446655440001", "--format", "vmdk"); err == nil {
		t.Fatal("expected an error for an invalid --format")
	}
	if _, ok := server.requestByMethod(http.MethodGet); ok {
		t.Fatal("no export request must be sent for an invalid --format")
	}
}

func TestVMExportNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rest/v0/vms/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	isolateVM(t, server.URL)

	_, err := runVM(t, "vm", "export", "550e8400-e29b-41d4-a716-446655440001", "--file", t.TempDir()+"/x.xva")
	if err == nil {
		t.Fatal("expected an error when the VM does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVMExportAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, ".xva") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"message":"boom"}`)
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rest/v0/vms/") {
			_, _ = fmt.Fprint(w, fixtureVM)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	isolateVM(t, server.URL)

	dir := t.TempDir()
	dst := dir + "/x.xva"
	if err := writeFile(dst, "yesterday"); err != nil {
		t.Fatal(err)
	}
	_, err := runVM(t, "vm", "export", "550e8400-e29b-41d4-a716-446655440001", "--file", dst)
	if err == nil {
		t.Fatal("expected an error when the export fails")
	}
	if !strings.Contains(err.Error(), "cannot export") {
		t.Fatalf("unexpected error: %v", err)
	}
	// A failed export must not touch an earlier archive with the same name.
	if data, err := os.ReadFile(dst); err != nil || string(data) != "yesterday" {
		t.Fatalf("the existing archive was changed: %q, %v", data, err)
	}
	assertOnlyFile(t, dir, "x.xva")
}

// TestVMExportCutStreamLeavesNoFile covers a transfer that breaks mid-way
// (a server or network failure, or a client timeout): no truncated archive
// may be left at the destination.
func TestVMExportCutStreamLeavesNoFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, ".xva") {
			// Promise more bytes than are sent, then drop the connection.
			w.Header().Set("Content-Length", "1000000")
			_, _ = fmt.Fprint(w, "XVA-PARTIAL")
			w.(http.Flusher).Flush()
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rest/v0/vms/") {
			_, _ = fmt.Fprint(w, fixtureVM)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	isolateVM(t, server.URL)

	dir := t.TempDir()
	_, err := runVM(t, "vm", "export", "550e8400-e29b-41d4-a716-446655440001", "--file", dir+"/web-01.xva")
	if err == nil {
		t.Fatal("expected an error when the stream is cut")
	}
	assertOnlyFile(t, dir)
}

// assertOnlyFile fails when dir holds anything but the given files (for
// example a truncated archive or a leftover temporary file).
func assertOnlyFile(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("files in the export directory: %v, want %v", got, want)
	}
}

// --- import -----------------------------------------------------------------

func TestVMImportFromFile(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	dir := t.TempDir()
	src := dir + "/web-01.xva"
	if err := writeFile(src, "XVA-ARCHIVE"); err != nil {
		t.Fatalf("cannot create source file: %v", err)
	}

	out, err := runVM(t, "vm", "import", src, "--pool", pool)
	if err != nil {
		t.Fatalf("vm import: %v", err)
	}
	if !strings.Contains(out, "9fe12ca3-d75d-cfb0-492e-cfd2bc6c568f") {
		t.Fatalf("import output should carry the new VM id:\n%s", out)
	}

	req, ok := server.requestByMethod(http.MethodPost)
	if !ok {
		t.Fatal("expected a POST import request")
	}
	if req.Path != "/rest/v0/pools/"+pool+"/vms" {
		t.Fatalf("unexpected import path: %s", req.Path)
	}
}

func TestVMImportWithSR(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	sr := "aaaaaaaa-bbbb-cccc-dddd-000000000002"
	src := t.TempDir() + "/web-01.xva"
	if err := writeFile(src, "XVA-ARCHIVE"); err != nil {
		t.Fatalf("cannot create source file: %v", err)
	}

	if _, err := runVM(t, "vm", "import", src, "--pool", pool, "--sr", sr); err != nil {
		t.Fatalf("vm import --sr: %v", err)
	}
	req, _ := server.requestByMethod(http.MethodPost)
	if req.Query != "sr="+sr {
		t.Fatalf("expected sr=%s in the query, got %q", sr, req.Query)
	}
}

func TestVMImportRequiresPool(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	src := t.TempDir() + "/web-01.xva"
	if err := writeFile(src, "XVA-ARCHIVE"); err != nil {
		t.Fatalf("cannot create source file: %v", err)
	}

	if _, err := runVM(t, "vm", "import", src); err == nil {
		t.Fatal("expected an error when --pool is missing")
	}
	if _, ok := server.requestByMethod(http.MethodPost); ok {
		t.Fatal("no import request must be sent without --pool")
	}
}

func TestVMImportBadPool(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	src := t.TempDir() + "/web-01.xva"
	if err := writeFile(src, "XVA-ARCHIVE"); err != nil {
		t.Fatalf("cannot create source file: %v", err)
	}

	if _, err := runVM(t, "vm", "import", src, "--pool", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid --pool id")
	}
}

func TestVMImportEmptyFile(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	src := t.TempDir() + "/empty.xva"
	if err := writeFile(src, ""); err != nil {
		t.Fatalf("cannot create source file: %v", err)
	}

	if _, err := runVM(t, "vm", "import", src, "--pool", pool); err == nil {
		t.Fatal("expected an error for an empty XVA file")
	}
	if _, ok := server.requestByMethod(http.MethodPost); ok {
		t.Fatal("no import request must be sent for an empty file")
	}
}

func TestVMImportMissingFile(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	if _, err := runVM(t, "vm", "import", "/does/not/exist.xva", "--pool", pool); err == nil {
		t.Fatal("expected an error when the source file does not exist")
	}
}

func TestVMImportPoolNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/rest/v0/pools/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	isolateVM(t, server.URL)

	pool := "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	src := t.TempDir() + "/web-01.xva"
	if err := writeFile(src, "XVA-ARCHIVE"); err != nil {
		t.Fatalf("cannot create source file: %v", err)
	}

	_, err := runVM(t, "vm", "import", src, "--pool", pool)
	if err == nil {
		t.Fatal("expected an error when the pool does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

// TestVMExportToStdoutKeepsStreamClean verifies that when the archive is
// piped to stdout, stdout carries only the binary archive and the
// confirmation message goes to stderr, so `xo vm export <id> > vm.xva`
// produces a clean file.
func TestVMExportToStdoutKeepsStreamClean(t *testing.T) {
	server := newXVAServer(t)
	defer server.Close()
	isolateVM(t, server.URL)

	root := newVMTestRoot()
	var stdout, stderr strings.Builder
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"vm", "export", "550e8400-e29b-41d4-a716-446655440001"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("vm export: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	if stdout.String() != "XVA-ARCHIVE" {
		t.Fatalf("stdout must carry only the archive, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "web-01") {
		t.Fatalf("the confirmation message must go to stderr, got %q", stderr.String())
	}
}
