package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExportWriter is the destination of an export: stdout, or a file that only
// appears once the export has fully succeeded.
type ExportWriter interface {
	io.Writer
	// Commit finishes a successful export: the file takes its final name.
	Commit() error
	// Abort discards a failed export: no partial file is left behind, and
	// an existing file at the destination is untouched.
	Abort()
}

// OpenExport opens the destination of an export. An empty or "-" path
// streams to stdout. Any other path is written to a temporary file in the
// same directory, renamed over the destination by Commit and removed by
// Abort, so a failed or interrupted export never leaves a truncated archive
// (nor destroys an earlier one with the same name).
func OpenExport(stdout io.Writer, path string) (ExportWriter, error) {
	if path == "" || path == "-" {
		return stdoutExport{stdout}, nil
	}
	dir, base := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, "."+base+".*.partial")
	if err != nil {
		return nil, fmt.Errorf("cannot create %q: %v", path, err)
	}
	return &fileExport{File: tmp, path: path}, nil
}

type stdoutExport struct{ io.Writer }

func (stdoutExport) Commit() error { return nil }
func (stdoutExport) Abort()        {}

type fileExport struct {
	*os.File
	path string
}

func (f *fileExport) Commit() error {
	if err := f.Sync(); err != nil {
		f.Abort()
		return fmt.Errorf("cannot write %q: %v", f.path, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return fmt.Errorf("cannot write %q: %v", f.path, err)
	}
	// os.CreateTemp uses 0600; give the archive the usual file mode.
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		_ = os.Remove(f.Name())
		return fmt.Errorf("cannot write %q: %v", f.path, err)
	}
	if err := os.Rename(f.Name(), f.path); err != nil {
		_ = os.Remove(f.Name())
		return fmt.Errorf("cannot write %q: %v", f.path, err)
	}
	return nil
}

func (f *fileExport) Abort() {
	_ = f.Close()
	_ = os.Remove(f.Name())
}
