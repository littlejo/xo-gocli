package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// referenceDir is the committed Markdown reference, relative to this package.
const referenceDir = "../../docs/reference/cli"

// TestMarkdownReferenceIsUpToDate regenerates the reference and compares it
// with the committed one, so a command or flag change cannot ship without
// its reference. Fix a failure with:
//
//	go run ./tools/gendocs -markdown docs/reference/cli
func TestMarkdownReferenceIsUpToDate(t *testing.T) {
	fresh := t.TempDir()
	if err := generateMarkdown(fresh); err != nil {
		t.Fatal(err)
	}

	want := readDir(t, fresh)
	got := readDir(t, referenceDir)
	for name, content := range want {
		committed, ok := got[name]
		switch {
		case !ok:
			t.Errorf("%s is missing from docs/reference/cli", name)
		case !bytes.Equal(committed, content):
			t.Errorf("%s is out of date", name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s is in docs/reference/cli but no longer generated", name)
		}
	}
	if t.Failed() {
		t.Log("regenerate with: go run ./tools/gendocs -markdown docs/reference/cli")
	}
}

func TestManPages(t *testing.T) {
	dir := t.TempDir()
	date := time.Date(2026, time.October, 20, 10, 0, 0, 0, time.UTC)
	if err := generateMan(dir, "1.2.3", &date); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(dir, "xo-vm-start.1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"xo vm start <id> [flags]", "xo 1.2.3", "Oct 2026", "EXAMPLE"} {
		if !bytes.Contains(page, []byte(want)) {
			t.Errorf("xo-vm-start.1 should contain %q", want)
		}
	}
}

func readDir(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(entries))
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" || e.Name() == "README.md" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = b
	}
	return files
}
