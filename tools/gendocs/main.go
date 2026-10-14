// Command gendocs generates the xo command reference from the cobra command
// tree, so the reference can never drift from --help:
//
//	go run ./tools/gendocs -markdown docs/reference/cli
//	go run ./tools/gendocs -man man -version 1.0.0 -date 2026-10-20T10:00:00Z
//
// The Markdown reference is committed (a test fails when it is stale); the
// man pages are built at release time and shipped in the archives.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/littlejo/xo-gocli/internal/commands"
)

func main() {
	markdownDir := flag.String("markdown", "", "write the Markdown reference to this directory")
	manDir := flag.String("man", "", "write the man pages to this directory")
	version := flag.String("version", "dev", "xo version printed in the man page footer")
	date := flag.String("date", "", "man page date, RFC 3339 (default: $SOURCE_DATE_EPOCH, else now)")
	flag.Parse()

	if *markdownDir == "" && *manDir == "" {
		fmt.Fprintln(os.Stderr, "gendocs: nothing to do, pass -markdown and/or -man")
		os.Exit(2)
	}
	if *markdownDir != "" {
		if err := generateMarkdown(*markdownDir); err != nil {
			fmt.Fprintln(os.Stderr, "gendocs:", err)
			os.Exit(1)
		}
	}
	if *manDir != "" {
		when, err := manDate(*date)
		if err != nil {
			fmt.Fprintln(os.Stderr, "gendocs:", err)
			os.Exit(2)
		}
		if err := generateMan(*manDir, *version, when); err != nil {
			fmt.Fprintln(os.Stderr, "gendocs:", err)
			os.Exit(1)
		}
	}
}

// manDate returns the man page date: the -date flag (RFC 3339) when given,
// else $SOURCE_DATE_EPOCH (Unix seconds, the reproducible-builds
// convention), else nil, which means the current time.
func manDate(flagValue string) (*time.Time, error) {
	if flagValue != "" {
		t, err := time.Parse(time.RFC3339, flagValue)
		if err != nil {
			return nil, fmt.Errorf("invalid -date: %w", err)
		}
		return &t, nil
	}
	if epoch := os.Getenv("SOURCE_DATE_EPOCH"); epoch != "" {
		secs, err := strconv.ParseInt(epoch, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid SOURCE_DATE_EPOCH %q: %w", epoch, err)
		}
		t := time.Unix(secs, 0).UTC()
		return &t, nil
	}
	return nil, nil
}

// generateMarkdown writes one page per command (xo.md, xo_vm.md,
// xo_vm_start.md, ...). The auto-generated date footer is disabled so the
// output only changes when the commands do.
func generateMarkdown(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	root := commands.NewRoot()
	root.DisableAutoGenTag = true
	return doc.GenMarkdownTree(root, dir)
}

// generateMan writes one man page per command (xo.1, xo-vm.1, ...), dated
// with date (see manDate); a nil date means the current time.
func generateMan(dir, version string, date *time.Time) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	root := commands.NewRoot()
	root.DisableAutoGenTag = true
	escapeAngles(root)
	return doc.GenManTree(root, &doc.GenManHeader{
		Title:   "XO",
		Section: "1",
		Source:  "xo " + version,
		Manual:  "xo manual",
		Date:    date,
	}, dir)
}

// escapeAngles backslash-escapes '<' and '>' in each command's usage line,
// one-line summary (the NAME section) and description. The man pages go through a Markdown renderer that would
// otherwise read placeholders such as '<id>' as HTML tags and drop them
// ('xo vm start  [flags]'). Examples are code blocks and need no escaping.
func escapeAngles(cmd *cobra.Command) {
	r := strings.NewReplacer("<", `\<`, ">", `\>`)
	cmd.Use = r.Replace(cmd.Use)
	cmd.Short = r.Replace(cmd.Short)
	cmd.Long = r.Replace(cmd.Long)
	for _, c := range cmd.Commands() {
		escapeAngles(c)
	}
}
