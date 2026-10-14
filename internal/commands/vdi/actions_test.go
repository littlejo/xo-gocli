package vdi

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/testenv"
)

// --- migrate -----------------------------------------------------------------

func TestVDIMigrate(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runVDI(t, "vdi", "migrate", getVDIID, "--sr", targetSRID)
	if err != nil {
		t.Fatalf("vdi migrate: %v", err)
	}
	if !strings.Contains(out, "system disk") || !strings.Contains(out, migrateTask) {
		t.Fatalf("migrate output should report the VDI name and task id:\n%s", out)
	}

	req, ok := server.requestByMethod(http.MethodPost)
	if !ok {
		t.Fatal("expected a POST migrate request")
	}
	if req.Path != "/rest/v0/vdis/"+getVDIID+"/actions/migrate" {
		t.Fatalf("unexpected migrate path: %s", req.Path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("cannot parse migrate body: %v\n%s", err, req.Body)
	}
	if body["srId"] != targetSRID {
		t.Fatalf("expected srId=%s, got %s", targetSRID, req.Body)
	}
}

func TestVDIMigrateJSON(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runVDI(t, "vdi", "migrate", getVDIID, "--sr", targetSRID, "--output", "json")
	if err != nil {
		t.Fatalf("vdi migrate --output json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("migrate JSON output is not valid JSON: %v\n%s", err, out)
	}
	if doc["task_id"] != migrateTask {
		t.Fatalf("unexpected migrate JSON payload: %s", out)
	}
}

func TestVDIMigrateRequiresSR(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	if _, err := runVDI(t, "vdi", "migrate", getVDIID); err == nil {
		t.Fatal("expected an error when --sr is missing")
	}
	if _, ok := server.requestByMethod(http.MethodPost); ok {
		t.Fatal("migrate must not be executed without --sr")
	}
}

func TestVDIMigrateBadSR(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	if _, err := runVDI(t, "vdi", "migrate", getVDIID, "--sr", "not-a-uuid"); err == nil {
		t.Fatal("expected an error for an invalid --sr id")
	}
	if _, ok := server.requestByMethod(http.MethodPost); ok {
		t.Fatal("migrate must not be executed with an invalid --sr id")
	}
}

func TestVDIMigrateNotFoundVDI(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	_, err := runVDI(t, "vdi", "migrate", missing, "--sr", targetSRID)
	if err == nil {
		t.Fatal("expected an error when the VDI does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := server.requestByMethod(http.MethodPost); ok {
		t.Fatal("migrate must not run when the VDI does not exist")
	}
}

// --- tag add / remove --------------------------------------------------------

func TestVDITagAdd(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runVDI(t, "vdi", "tag", "add", getVDIID, "production")
	if err != nil {
		t.Fatalf("vdi tag add: %v", err)
	}
	if !strings.Contains(out, `Tag "production" added on VDI "system disk"`) {
		t.Fatalf("tag add output missing confirmation:\n%s", out)
	}
	req, ok := server.requestByMethod(http.MethodPut)
	if !ok {
		t.Fatal("expected a PUT request")
	}
	if req.Path != "/rest/v0/vdis/"+getVDIID+"/tags/production" {
		t.Fatalf("unexpected tag add path: %s", req.Path)
	}
}

func TestVDITagRemove(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runVDI(t, "vdi", "tag", "remove", getVDIID, "production")
	if err != nil {
		t.Fatalf("vdi tag remove: %v", err)
	}
	if !strings.Contains(out, `Tag "production" removed on VDI "system disk"`) {
		t.Fatalf("tag remove output missing confirmation:\n%s", out)
	}
	req, ok := server.requestByMethod(http.MethodDelete)
	if !ok {
		t.Fatal("expected a DELETE request")
	}
	if req.Path != "/rest/v0/vdis/"+getVDIID+"/tags/production" {
		t.Fatalf("unexpected tag remove path: %s", req.Path)
	}
}

func TestVDITagNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	_, err := runVDI(t, "vdi", "tag", "add", missing, "production")
	if err == nil {
		t.Fatal("expected an error when the VDI does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := server.requestByMethod(http.MethodPut); ok {
		t.Fatal("tag add must not run when the VDI does not exist")
	}
}

func TestVDITagInvalidID(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	if _, err := runVDI(t, "vdi", "tag", "add", "not-a-uuid", "production"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
	if _, ok := server.requestByMethod(http.MethodPut); ok {
		t.Fatal("tag add must not run with an invalid id")
	}
}

// --- export ------------------------------------------------------------------

// TestVDIExportToFile writes to a temp file so the streamed bytes can be
// inspected; the human confirmation line is checked separately.
func TestVDIExportToFile(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	dest := t.TempDir() + "/disk.raw"
	out, err := runVDI(t, "vdi", "export", getVDIID, "--file", dest)
	if err != nil {
		t.Fatalf("vdi export: %v", err)
	}
	data, err := readFile(dest)
	if err != nil {
		t.Fatalf("cannot read exported file: %v", err)
	}
	if string(data) != fixtureImage {
		t.Fatalf("expected exported content %q, got %q", fixtureImage, string(data))
	}
	if !strings.Contains(out, "Exported VDI") || !strings.Contains(out, "system disk") {
		t.Fatalf("export output missing confirmation:\n%s", out)
	}
}

func TestVDIExportStdout(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	// To stdout, the binary stream goes to stdout and the message to stderr,
	// so out contains both.
	out, err := runVDI(t, "vdi", "export", getVDIID)
	if err != nil {
		t.Fatalf("vdi export: %v", err)
	}
	if !strings.Contains(out, fixtureImage) {
		t.Fatalf("stdout should carry the image bytes:\n%s", out)
	}
	if !strings.Contains(out, "Exported VDI") {
		t.Fatalf("stderr should carry the confirmation:\n%s", out)
	}
}

func TestVDIExportVHD(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	dest := t.TempDir() + "/disk.vhd"
	if _, err := runVDI(t, "vdi", "export", getVDIID, "--format", "vhd", "--file", dest); err != nil {
		t.Fatalf("vdi export --format vhd: %v", err)
	}
	data, err := readFile(dest)
	if err != nil {
		t.Fatalf("cannot read exported file: %v", err)
	}
	if string(data) != fixtureImage {
		t.Fatalf("expected exported content %q, got %q", fixtureImage, string(data))
	}
}

func TestVDIExportInvalidFormat(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	if _, err := runVDI(t, "vdi", "export", getVDIID, "--format", "zip"); err == nil {
		t.Fatal("expected an error for an invalid --format")
	}
}

func TestVDIExportNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	if _, err := runVDI(t, "vdi", "export", missing); err == nil {
		t.Fatal("expected an error when the VDI does not exist")
	}
}

// --- import ------------------------------------------------------------------

func TestVDIImportFromFile(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	src := writeTempFile(t, fixtureImage)
	out, err := runVDI(t, "vdi", "import", getVDIID, src, "--yes")
	if err != nil {
		t.Fatalf("vdi import: %v", err)
	}
	if !strings.Contains(out, "Imported image into VDI") || !strings.Contains(out, "system disk") {
		t.Fatalf("import output missing confirmation:\n%s", out)
	}
	req, ok := server.requestByMethod(http.MethodPut)
	if !ok {
		t.Fatal("expected a PUT import request")
	}
	if req.Path != "/rest/v0/vdis/"+getVDIID+".raw" {
		t.Fatalf("unexpected import path: %s", req.Path)
	}
	if req.Body != fixtureImage {
		t.Fatalf("expected body %q, got %q", fixtureImage, req.Body)
	}
}

func TestVDIImportVHD(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	src := writeTempFile(t, fixtureImage)
	if _, err := runVDI(t, "vdi", "import", getVDIID, src, "--format", "vhd", "--yes"); err != nil {
		t.Fatalf("vdi import --format vhd: %v", err)
	}
	req, _ := server.requestByMethod(http.MethodPut)
	if req.Path != "/rest/v0/vdis/"+getVDIID+".vhd" {
		t.Fatalf("unexpected import path: %s", req.Path)
	}
}

func TestVDIImportRequiresConfirmation(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	src := writeTempFile(t, fixtureImage)
	_, err := runVDI(t, "vdi", "import", getVDIID, src)
	if err == nil {
		t.Fatal("expected an error when no confirmation is given and stdin is not a terminal")
	}
	if _, ok := server.requestByMethod(http.MethodPut); ok {
		t.Fatal("the VDI must not be imported without confirmation")
	}
}

func TestVDIImportSkipsConfirmationWithEnvYes(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)
	t.Setenv("XOA_YES", "1")

	src := writeTempFile(t, fixtureImage)
	if _, err := runVDI(t, "vdi", "import", getVDIID, src); err != nil {
		t.Fatalf("vdi import with XOA_YES=1: %v", err)
	}
	if _, ok := server.requestByMethod(http.MethodPut); !ok {
		t.Fatal("expected a PUT import request when XOA_YES=1")
	}
}

func TestVDIImportNotFound(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	src := writeTempFile(t, fixtureImage)
	_, err := runVDI(t, "vdi", "import", missing, src, "--yes")
	if err == nil {
		t.Fatal("expected an error when the VDI does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := server.requestByMethod(http.MethodPut); ok {
		t.Fatal("the VDI must not be imported when it does not exist")
	}
}

func TestVDIImportEmptyFile(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	src := writeTempFile(t, "")
	if _, err := runVDI(t, "vdi", "import", getVDIID, src, "--yes"); err == nil {
		t.Fatal("expected an error for an empty image")
	}
	if _, ok := server.requestByMethod(http.MethodPut); ok {
		t.Fatal("the VDI must not be imported from an empty file")
	}
}

func TestVDIImportInvalidFormat(t *testing.T) {
	server := newMutationServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	src := writeTempFile(t, fixtureImage)
	if _, err := runVDI(t, "vdi", "import", getVDIID, src, "--format", "zip", "--yes"); err == nil {
		t.Fatal("expected an error for an invalid --format")
	}
	if _, ok := server.requestByMethod(http.MethodPut); ok {
		t.Fatal("import must not run with an invalid --format")
	}
}

// --- helpers -----------------------------------------------------------------

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := t.TempDir() + "/image"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("cannot write temp image: %v", err)
	}
	return path
}
