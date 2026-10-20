package taskwait

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
)

func startedCmd(t *testing.T, format string) (*cobra.Command, *strings.Builder) {
	t.Helper()
	cmd := &cobra.Command{Use: "start"}
	cmd.Flags().String(cli.FlagOutput, "table", "")
	if err := cmd.Flags().Set(cli.FlagOutput, format); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	cmd.SetOut(&out)
	return cmd, &out
}

func TestRenderStartedHuman(t *testing.T) {
	for _, tc := range []struct{ taskID, want string }{
		{"t-1", `Requested start of "web-01" (task t-1)`},
		{"", `Requested start of "web-01"`},
	} {
		cmd, out := startedCmd(t, "table")
		if err := RenderStarted(cmd, "vm", "start", "web-01", `"web-01"`, tc.taskID); err != nil {
			t.Fatalf("RenderStarted: %v", err)
		}
		if strings.TrimSpace(out.String()) != tc.want {
			t.Fatalf("output = %q, want %q", out.String(), tc.want)
		}
	}
}

func TestRenderStartedJSON(t *testing.T) {
	cmd, out := startedCmd(t, "json")
	if err := RenderStarted(cmd, "vm", "start", "web-01", `"web-01"`, "t-1"); err != nil {
		t.Fatalf("RenderStarted: %v", err)
	}
	for _, want := range []string{`"action": "start"`, `"vm": "web-01"`, `"task_id": "t-1"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("JSON misses %s:\n%s", want, out.String())
		}
	}

	cmd, out = startedCmd(t, "json")
	if err := RenderStarted(cmd, "vbd", "connect", "xvda", "xvda", ""); err != nil {
		t.Fatalf("RenderStarted: %v", err)
	}
	if strings.Contains(out.String(), "task_id") {
		t.Fatalf("a synchronous action must not print a task id:\n%s", out.String())
	}
}

func TestAddFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "start"}
	AddFlag(cmd)
	if err := cmd.Flags().Set(FlagWait, "true"); err != nil {
		t.Fatal(err)
	}
	if !cli.WaitEnabled(cmd) {
		t.Fatal("--wait set: WaitEnabled should be true")
	}
}
