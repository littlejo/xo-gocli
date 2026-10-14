package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenExportCommitWritesTheFile(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "vm.xva")
	w, err := OpenExport(nil, dst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("archive")); err != nil {
		t.Fatal(err)
	}
	// Nothing at the destination until the export is committed.
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("the destination should not exist before Commit: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "archive" {
		t.Fatalf("got %q, %v", data, err)
	}
	assertNoPartialFiles(t, filepath.Dir(dst))
}

// A failed export leaves no partial file, and an earlier archive with the
// same name is untouched.
func TestOpenExportAbortKeepsTheExistingFile(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "vm.xva")
	if err := os.WriteFile(dst, []byte("yesterday"), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := OpenExport(nil, dst)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("trunc"))
	w.Abort()

	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "yesterday" {
		t.Fatalf("the existing archive was changed: %q, %v", data, err)
	}
	assertNoPartialFiles(t, filepath.Dir(dst))
}

func TestOpenExportStdout(t *testing.T) {
	for _, path := range []string{"", "-"} {
		var out bytes.Buffer
		w, err := OpenExport(&out, path)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("data"))
		if err := w.Commit(); err != nil || out.String() != "data" {
			t.Fatalf("%q: got %q, %v", path, out.String(), err)
		}
	}
}

func assertNoPartialFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".*.partial"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) > 0 {
		t.Fatalf("partial files left behind: %v", matches)
	}
}
