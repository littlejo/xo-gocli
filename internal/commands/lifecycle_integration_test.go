//go:build integration

package commands

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// This test runs against a real Xen Orchestra instance when the
// XOA_TEST_URL and XOA_TEST_TOKEN environment variables are set. It is
// explicitly enabled with:
//
//	go test -tags=integration ./...
//
// In CI it is pointed at the xo-api-sim simulator, which ships pool and
// template fixtures and answers the VM lifecycle endpoints, so the test
// exercises a real create -> start -> stop -> snapshot cycle with no live
// instance and no credential. Without the variables the test is reported as
// skipped, never as passed.
//
// Unlike the per-resource list tests, this test drives the whole command
// tree (pool list, template list, vm *, task list) end to end.
//
// The VM lifecycle covered is: create -> start -> pause -> unpause ->
// suspend -> resume -> stop -> snapshot -> tags -> delete, so every VM
// action exposed by the CLI is exercised against the real REST endpoints.

func TestIntegrationVMLifecycle(t *testing.T) {
	url := os.Getenv("XOA_TEST_URL")
	token := os.Getenv("XOA_TEST_TOKEN")
	if url == "" || token == "" {
		t.Skip("integration test skipped: set XOA_TEST_URL and XOA_TEST_TOKEN to run against a real Xen Orchestra instance")
	}

	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE"} {
		t.Setenv(key, "")
	}
	t.Setenv("XOA_ENDPOINT", url)
	t.Setenv("XOA_TOKEN", token)

	// run executes the full xo command tree with the given arguments and
	// returns (combined stdout, error). A fresh root is used per call so the
	// command's flag state never leaks between invocations. Stdin is an
	// explicit non-terminal reader, so destructive commands that lack --yes
	// fail deterministically instead of blocking on a prompt (even if the
	// test itself runs under a terminal).
	run := func(args ...string) (string, error) {
		t.Helper()
		root := NewRoot()
		root.SetIn(strings.NewReader(""))
		var out strings.Builder
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		err := root.ExecuteContext(context.Background())
		return out.String(), err
	}

	// 1. Pick a pool with a default SR (create_vm builds the template's
	// disks on it) and a VM template to base the VM on.
	poolOut, err := run("pool", "list", "--output", "json")
	if err != nil {
		t.Fatalf("xo pool list: %v\n%s", err, poolOut)
	}
	var pools []map[string]any
	if err := json.Unmarshal([]byte(poolOut), &pools); err != nil {
		t.Fatalf("pool list output is not valid JSON: %v\n%s", err, poolOut)
	}
	poolID := ""
	for _, p := range pools {
		if id, _ := p["id"].(string); id != "" {
			poolID = id
			break
		}
	}
	if poolID == "" {
		t.Skip("no pools available on the test instance")
	}

	tmplOut, err := run("template", "list", "--output", "json")
	if err != nil {
		t.Fatalf("xo template list: %v\n%s", err, tmplOut)
	}
	var templates []map[string]any
	if err := json.Unmarshal([]byte(tmplOut), &templates); err != nil {
		t.Fatalf("template list output is not valid JSON: %v\n%s", err, tmplOut)
	}
	templateID := ""
	for _, tpl := range templates {
		if id, _ := tpl["id"].(string); id != "" {
			templateID = id
			break
		}
	}
	if templateID == "" {
		t.Skip("no VM templates available on the test instance")
	}

	// 2. Create a VM and check the returned object.
	const name = "ci-func-test"
	createOut, err := run("vm", "create", name, "--pool", poolID, "--template", templateID, "--output", "json")
	if err != nil {
		t.Fatalf("xo vm create: %v\n%s", err, createOut)
	}
	var created map[string]any
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("create output is not valid JSON: %v\n%s", err, createOut)
	}
	vmID, _ := created["id"].(string)
	if vmID == "" {
		t.Fatalf("created VM has no id: %s", createOut)
	}
	if created["name_label"] != name {
		t.Fatalf("created VM name = %v, want %s", created["name_label"], name)
	}
	if created["power_state"] != "Halted" {
		t.Fatalf("created VM power_state = %v, want Halted", created["power_state"])
	}
	t.Logf("created VM %s (%s)", name, vmID)

	// 3. The new VM shows up in 'vm list', found via a JMESPath filter.
	listOut, err := run("vm", "list", "--output", "json", "--query", "[?name_label==`"+name+"`].id")
	if err != nil {
		t.Fatalf("xo vm list: %v\n%s", err, listOut)
	}
	var ids []string
	if err := json.Unmarshal([]byte(listOut), &ids); err != nil {
		t.Fatalf("vm list query output is not valid JSON: %v\n%s", err, listOut)
	}
	found := false
	for _, id := range ids {
		if id == vmID {
			found = true
		}
	}
	if !found {
		t.Fatalf("vm list --query did not return the created VM: got %v, want %s among them", ids, vmID)
	}

	// 3b. The same data must render in the human formats (default table and
	// --output text). These use the generic list renderers, and a regression
	// there used to print nothing for list results — the JSON assertion above
	// would not have caught it.
	tableOut, err := run("vm", "list")
	if err != nil {
		t.Fatalf("xo vm list (table): %v\n%s", err, tableOut)
	}
	if !strings.Contains(tableOut, name) {
		t.Fatalf("table output should show the created VM %q:\n%s", name, tableOut)
	}
	textOut, err := run("vm", "list", "--output", "text")
	if err != nil {
		t.Fatalf("xo vm list (text): %v\n%s", err, textOut)
	}
	if !strings.Contains(textOut, name) {
		t.Fatalf("text output should show the created VM %q:\n%s", name, textOut)
	}

	// 4. Start the VM; the state must be reflected on re-read.
	startOut, err := run("vm", "start", vmID)
	if err != nil {
		t.Fatalf("xo vm start: %v\n%s", err, startOut)
	}
	if !strings.Contains(startOut, "(task ") {
		t.Fatalf("start output should carry a task id:\n%s", startOut)
	}
	stateOut, err := run("vm", "get", vmID, "--output", "json", "--query", "power_state")
	if err != nil {
		t.Fatalf("xo vm get after start: %v\n%s", err, stateOut)
	}
	var state string
	if err := json.Unmarshal([]byte(stateOut), &state); err != nil {
		t.Fatalf("power_state is not a JSON string: %v\n%s", err, stateOut)
	}
	if state != "Running" {
		t.Fatalf("power_state after start = %q, want Running", state)
	}

	// The single-object view must also render in the human formats.
	getOut, err := run("vm", "get", vmID)
	if err != nil {
		t.Fatalf("xo vm get (table): %v\n%s", err, getOut)
	}
	if !strings.Contains(getOut, name) {
		t.Fatalf("vm get table output should show the VM name %q:\n%s", name, getOut)
	}

	// 4b. Pause / unpause round trip: pause is a reversible action, so it
	// runs without confirmation, and the power state must round-trip.
	pauseOut, err := run("vm", "pause", vmID)
	if err != nil {
		t.Fatalf("xo vm pause: %v\n%s", err, pauseOut)
	}
	if !strings.Contains(pauseOut, "(task ") {
		t.Fatalf("pause output should carry a task id:\n%s", pauseOut)
	}
	stateOut, err = run("vm", "get", vmID, "--output", "json", "--query", "power_state")
	if err != nil {
		t.Fatalf("xo vm get after pause: %v\n%s", err, stateOut)
	}
	if err := json.Unmarshal([]byte(stateOut), &state); err != nil {
		t.Fatalf("power_state is not a JSON string: %v\n%s", err, stateOut)
	}
	if state != "Paused" {
		t.Fatalf("power_state after pause = %q, want Paused", state)
	}

	if _, err := run("vm", "unpause", vmID); err != nil {
		t.Fatalf("xo vm unpause: %v", err)
	}
	stateOut, err = run("vm", "get", vmID, "--output", "json", "--query", "power_state")
	if err != nil {
		t.Fatalf("xo vm get after unpause: %v\n%s", err, stateOut)
	}
	if err := json.Unmarshal([]byte(stateOut), &state); err != nil {
		t.Fatalf("power_state is not a JSON string: %v\n%s", err, stateOut)
	}
	if state != "Running" {
		t.Fatalf("power_state after unpause = %q, want Running", state)
	}

	// 4c. Suspend / resume round trip, same shape as pause / unpause.
	suspendOut, err := run("vm", "suspend", vmID)
	if err != nil {
		t.Fatalf("xo vm suspend: %v\n%s", err, suspendOut)
	}
	if !strings.Contains(suspendOut, "(task ") {
		t.Fatalf("suspend output should carry a task id:\n%s", suspendOut)
	}
	stateOut, err = run("vm", "get", vmID, "--output", "json", "--query", "power_state")
	if err != nil {
		t.Fatalf("xo vm get after suspend: %v\n%s", err, stateOut)
	}
	if err := json.Unmarshal([]byte(stateOut), &state); err != nil {
		t.Fatalf("power_state is not a JSON string: %v\n%s", err, stateOut)
	}
	if state != "Suspended" {
		t.Fatalf("power_state after suspend = %q, want Suspended", state)
	}

	if _, err := run("vm", "resume", vmID); err != nil {
		t.Fatalf("xo vm resume: %v", err)
	}
	stateOut, err = run("vm", "get", vmID, "--output", "json", "--query", "power_state")
	if err != nil {
		t.Fatalf("xo vm get after resume: %v\n%s", err, stateOut)
	}
	if err := json.Unmarshal([]byte(stateOut), &state); err != nil {
		t.Fatalf("power_state is not a JSON string: %v\n%s", err, stateOut)
	}
	if state != "Running" {
		t.Fatalf("power_state after resume = %q, want Running", state)
	}

	// 5. Stop no longer asks for confirmation (S7): it must proceed without
	// --yes, even though stdin is not a terminal.
	stopOut, err := run("vm", "stop", vmID)
	if err != nil {
		t.Fatalf("xo vm stop: %v\n%s", err, stopOut)
	}
	stateOut, err = run("vm", "get", vmID, "--output", "json", "--query", "power_state")
	if err != nil {
		t.Fatalf("xo vm get after stop: %v\n%s", err, stateOut)
	}
	if err := json.Unmarshal([]byte(stateOut), &state); err != nil {
		t.Fatalf("power_state is not a JSON string: %v\n%s", err, stateOut)
	}
	if state != "Halted" {
		t.Fatalf("power_state after stop = %q, want Halted", state)
	}

	// 6. Take a snapshot; it must be listed on the VM.
	snapOut, err := run("vm", "snapshot", vmID, "--name", "func-snap")
	if err != nil {
		t.Fatalf("xo vm snapshot: %v\n%s", err, snapOut)
	}
	if !strings.Contains(snapOut, "(task ") {
		t.Fatalf("snapshot output should carry a task id:\n%s", snapOut)
	}
	snapOut2, err := run("vm", "get", vmID, "--output", "json", "--query", "snapshots")
	if err != nil {
		t.Fatalf("xo vm get snapshots: %v\n%s", err, snapOut2)
	}
	var snapshots []string
	if err := json.Unmarshal([]byte(snapOut2), &snapshots); err != nil {
		t.Fatalf("snapshots is not a JSON list: %v\n%s", err, snapOut2)
	}
	if len(snapshots) == 0 {
		t.Fatalf("expected at least one snapshot on the VM, got %s", snapOut2)
	}

	// 7. The async operations must have produced tasks.
	taskOut, err := run("task", "list", "--output", "json", "--query", "length(@)")
	if err != nil {
		t.Fatalf("xo task list: %v\n%s", err, taskOut)
	}
	var count int
	if err := json.Unmarshal([]byte(strings.TrimSpace(taskOut)), &count); err != nil {
		t.Fatalf("task count is not a JSON number: %v\n%s", err, taskOut)
	}
	if count < 8 {
		t.Fatalf("expected at least 8 tasks (create, start, pause, unpause, suspend, resume, stop, snapshot), got %d", count)
	}

	// Note: 'vm update' is intentionally not exercised here. Its PATCH body
	// uses the camelCase field names of the official API (nameLabel, ...),
	// while the simulator's generic PATCH handler only merges the body
	// verbatim, so it cannot reflect the rename on re-read. The update path
	// is pinned by the unit tests instead.

	// 8. Tag add/remove round trip.
	tagOut, err := run("vm", "tag", "add", vmID, "ci-func")
	if err != nil {
		t.Fatalf("xo vm tag add: %v\n%s", err, tagOut)
	}
	tagsOut, err := run("vm", "get", vmID, "--output", "json", "--query", "tags")
	if err != nil {
		t.Fatalf("xo vm get tags: %v\n%s", err, tagsOut)
	}
	var tags []string
	if err := json.Unmarshal([]byte(tagsOut), &tags); err != nil {
		t.Fatalf("tags is not a JSON list: %v\n%s", err, tagsOut)
	}
	var tagFound bool
	for _, tag := range tags {
		if tag == "ci-func" {
			tagFound = true
		}
	}
	if !tagFound {
		t.Fatalf("tag ci-func not present after add: %s", tagsOut)
	}
	if _, err := run("vm", "tag", "remove", vmID, "ci-func"); err != nil {
		t.Fatalf("xo vm tag remove: %v", err)
	}

	// 9. Delete is destructive: without --yes and without a terminal it must
	// refuse to run, and with --yes it must remove the VM.
	if out, err := run("vm", "delete", vmID); err == nil {
		t.Fatalf("delete without --yes must not proceed without a terminal:\n%s", out)
	}
	if _, err := run("vm", "delete", vmID, "--yes"); err != nil {
		t.Fatalf("xo vm delete --yes: %v", err)
	}
	// The root command silences errors, so the not-found message is on err,
	// not in the (empty) output.
	if _, err := run("vm", "get", vmID); err == nil {
		t.Fatal("the deleted VM must not be retrievable")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("get on a deleted VM should report not found: %v", err)
	}
}
