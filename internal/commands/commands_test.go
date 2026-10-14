package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// TestRootOutputShorthand pins the -o shortcut for --output. The long form is
// exercised end-to-end by the per-resource list tests; this guards the wiring
// of the shortcut on the real root so it cannot silently regress to a
// long-only flag.
func TestRootOutputShorthand(t *testing.T) {
	root := NewRoot()
	f := root.PersistentFlags().Lookup(cli.FlagOutput)
	if f == nil {
		t.Fatalf("--%s is not defined on the root command", cli.FlagOutput)
	}
	if f.Shorthand != "o" {
		t.Fatalf("--%s shorthand = %q, want %q", cli.FlagOutput, f.Shorthand, "o")
	}
}

// TestRootTimeoutFlag pins that the global --timeout flag is defined on the
// real root (so it is available to every command) and is a duration. The
// precedence rules are covered by the cli package tests; this guards the
// wiring on NewRoot so the flag cannot silently disappear.
func TestRootTimeoutFlag(t *testing.T) {
	root := NewRoot()
	f := root.PersistentFlags().Lookup(cli.FlagTimeout)
	if f == nil {
		t.Fatalf("--%s is not defined on the root command", cli.FlagTimeout)
	}
	if f.Value.Type() != "duration" {
		t.Fatalf("--%s type = %q, want %q", cli.FlagTimeout, f.Value.Type(), "duration")
	}
}

// TestVersionCommand pins that 'xo version' exists and prints the same line
// as '--version', offline (no profile or connection needed).
func TestVersionCommand(t *testing.T) {
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("xo version: %v", err)
	}
	want := "xo version " + cli.Version
	if got := strings.TrimSpace(out.String()); got != want {
		t.Fatalf("xo version = %q, want %q", got, want)
	}
}

// TestVersionFlagHasNoShorthand pins that --version works and that -v is not
// taken: -v is kept free (e.g. for a future --verbose).
func TestVersionFlagHasNoShorthand(t *testing.T) {
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("xo --version: %v", err)
	}
	if got, want := strings.TrimSpace(out.String()), "xo version "+cli.Version; got != want {
		t.Fatalf("xo --version = %q, want %q", got, want)
	}

	root = NewRoot()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"-v"})
	if err := root.Execute(); err == nil {
		t.Fatal("xo -v should be an unknown shorthand, got no error")
	}
}

// TestWaitTimeoutDoesNotShadowGlobal pins that the commands with a wait
// deadline name it --wait-timeout, so the global --timeout keeps its
// per-request meaning there instead of being shadowed by a local flag.
func TestWaitTimeoutDoesNotShadowGlobal(t *testing.T) {
	root := NewRoot()
	for _, path := range [][]string{
		{"task", "wait"},
		{"pool", "rolling-update"},
		{"pool", "rolling-reboot"},
		{"pool", "emergency-shutdown"},
	} {
		cmd, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("%v: %v", path, err)
		}
		if cmd.LocalNonPersistentFlags().Lookup(cli.FlagTimeout) != nil {
			t.Errorf("%v defines a local --%s that shadows the global one", path, cli.FlagTimeout)
		}
		if cmd.LocalNonPersistentFlags().Lookup(cli.FlagWaitTimeout) == nil {
			t.Errorf("%v has no --%s flag", path, cli.FlagWaitTimeout)
		}
	}
}

// TestRootRestFlagMerge guards against the 'xo rest' panic (blocker B1 in
// docs/archive/road-to-v1.md): when a local subcommand flag reuses a shorthand the
// root persistent flags already own, cobra panics in mergePersistentFlags on
// the first invocation — even 'rest --help'. The per-package rest tests
// build their own minimal root without the global shorthands, so the
// collision only surfaced in the real tree.
func TestRootRestFlagMerge(t *testing.T) {
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"rest", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("rest --help on the real root: %v", err)
	}
	if !strings.Contains(out.String(), "--data") {
		t.Fatal("rest --help should document the --data flag")
	}
}
