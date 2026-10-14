//go:build integration

package commands

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// These tests pin the use cases documented in docs/usecases.md against a real
// Xen Orchestra instance when the XOA_TEST_URL and XOA_TEST_TOKEN environment
// variables are set, explicitly enabled with:
//
//	go test -tags=integration ./...
//
// In CI they are pointed at the xo-api-sim simulator (the 'functional' job of
// .github/workflows/ci.yml), which answers every REST endpoint the use cases
// use, so each documented flow is exercised end to end with real HTTP requests
// and no live instance and no credential. Without the variables the tests are
// reported as skipped, never as passed.
//
// Every step of a use case that is a typed 'xo' command must be covered here;
// the steps that are not (guest-internal work such as mkfs/mount, and the VM
// -> template conversion, which has no REST endpoint and is done in the web
// UI) cannot be, and are documented as such below.

const envSkipMessage = "integration test skipped: set XOA_TEST_URL and XOA_TEST_TOKEN to run against a real Xen Orchestra instance"

// setupUseCaseEnv points the command tree at the test instance (or simulator)
// through the environment, with a throwaway config file so the developer's
// local profiles can never leak in.
func setupUseCaseEnv(t *testing.T) (url, token string) {
	t.Helper()
	url = os.Getenv("XOA_TEST_URL")
	token = os.Getenv("XOA_TEST_TOKEN")
	if url == "" || token == "" {
		t.Skip(envSkipMessage)
	}
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE", "XOA_YES", "XOA_WAIT"} {
		t.Setenv(key, "")
	}
	t.Setenv("XOA_ENDPOINT", url)
	t.Setenv("XOA_TOKEN", token)
	return url, token
}

// newUseCaseRun returns a runner for the full xo command tree. A fresh root
// is used per call so a command's flag state never leaks between
// invocations; stdin is an explicit non-terminal reader, so destructive
// commands that lack --yes fail deterministically instead of blocking.
func newUseCaseRun(t *testing.T) func(args ...string) (string, error) {
	t.Helper()
	return func(args ...string) (string, error) {
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
}

// runJSON runs the command and unmarshals its (JSON) stdout into v, failing
// the test on any error: every step of a use case must emit valid machine
// output, not merely succeed.
func runJSON(t *testing.T, run func(...string) (string, error), v any, args ...string) {
	t.Helper()
	out, err := run(args...)
	if err != nil {
		t.Fatalf("xo %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	if err := json.Unmarshal([]byte(out), v); err != nil {
		t.Fatalf("xo %s output is not valid JSON: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// pickTestEnvironment returns a (pool, template, default SR) triple that a
// use case can be run on: the pool must have a default_SR (create_vm builds
// the template's disks on it) and at least one VM template that belongs to it
// (a template from another pool is refused by create_vm). The template id is
// the one printed by 'template list'; 'vm create' also accepts a composite
// <poolId>-<uuid> id, so either shape works.
func pickTestEnvironment(t *testing.T, run func(...string) (string, error)) (poolID, templateID, srID string) {
	t.Helper()
	var pools []map[string]any
	runJSON(t, run, &pools, "pool", "list", "--output", "json")
	var templates []map[string]any
	runJSON(t, run, &templates, "template", "list", "--output", "json")

	for _, p := range pools {
		poolID, _ = p["id"].(string)
		srID, _ = p["default_SR"].(string)
		if poolID == "" || srID == "" {
			continue
		}
		// The pool reference of a template is $pool on the simulator and
		// $poolId on most real instances; accept both.
		for _, tpl := range templates {
			if tpl["$pool"] == poolID || tpl["$poolId"] == poolID {
				if id, _ := tpl["id"].(string); id != "" {
					return poolID, id, srID
				}
			}
		}
	}
	t.Skip("no pool with a default SR and a VM template on the test instance")
	return "", "", ""
}

// createTestVM creates a VM from the given template in the given pool and
// returns its id.
func createTestVM(t *testing.T, run func(...string) (string, error), name, poolID, templateID string) string {
	t.Helper()
	var created map[string]any
	runJSON(t, run, &created, "vm", "create", name, "--pool", poolID, "--template", templateID, "--output", "json")
	vmID, _ := created["id"].(string)
	if vmID == "" {
		t.Fatalf("created VM has no id")
	}
	t.Cleanup(func() {
		if _, err := run("vm", "delete", vmID, "--yes"); err != nil {
			t.Logf("cleanup: xo vm delete %s: %v", vmID, err)
		}
	})
	return vmID
}

// TestIntegrationUseCaseAddDiskToVM pins the "Add a disk to a VM" use case
// (docs/usecases.md) end to end with typed commands:
//
//  1. find a target SR        xo sr list / pool's default_SR
//  2. create the VDI          xo vdi create
//  3. attach it (VBD)         xo vbd create
//  4. hot-plug it             xo vbd connect / xo vbd disconnect
//  5. verify                  xo vm vdis / xo vbd list
//     (remove) in reverse order  xo vbd delete / xo vdi delete
//
// Step 6 of the use case (mkfs/mount inside the guest) is out of reach for a
// CLI test: it happens in the guest OS, not in Xen Orchestra.
func TestIntegrationUseCaseAddDiskToVM(t *testing.T) {
	setupUseCaseEnv(t)
	run := newUseCaseRun(t)
	poolID, templateID, srID := pickTestEnvironment(t, run)

	// Step 1: the SR list must show the target SR.
	var srs []map[string]any
	runJSON(t, run, &srs, "sr", "list", "--output", "json")
	srFound := false
	for _, sr := range srs {
		if sr["id"] == srID {
			srFound = true
		}
	}
	if !srFound {
		t.Fatalf("the pool's default SR %s is not in 'sr list'", srID)
	}

	// The VM the disk will be attached to.
	vmID := createTestVM(t, run, "uc-disk-vm", poolID, templateID)

	// Step 2: create the VDI on the SR.
	var vdiOut map[string]any
	runJSON(t, run, &vdiOut, "vdi", "create", "uc-data-01", "--sr", srID, "--size", "10G", "--output", "json")
	vdiID, _ := vdiOut["vdi"].(string)
	if vdiID == "" {
		t.Fatalf("vdi create output has no vdi id: %s", vdiOut)
	}
	// If a later step fails, the disk must not be left behind (the VBD, if
	// any, is detached first — a VDI that is still attached cannot be
	// deleted).
	t.Cleanup(func() {
		if out, err := run("vbd", "list", "--vm", vmID, "--output", "json"); err == nil {
			var leftovers []map[string]any
			if err := json.Unmarshal([]byte(out), &leftovers); err == nil {
				for _, v := range leftovers {
					if v["VDI"] == vdiID {
						if id, _ := v["id"].(string); id != "" {
							if _, derr := run("vbd", "delete", id, "--yes"); derr != nil {
								t.Logf("cleanup: xo vbd delete %s: %v", id, derr)
							}
						}
					}
				}
			}
		}
		if _, err := run("vdi", "delete", vdiID, "--yes"); err != nil {
			t.Logf("cleanup: xo vdi delete %s: %v", vdiID, err)
		}
	})

	// The VDI is not attached to anything yet: it must appear in 'vdi list'.
	var vdiList []map[string]any
	runJSON(t, run, &vdiList, "vdi", "list", "--output", "json")
	vdiListed := false
	for _, v := range vdiList {
		if v["id"] == vdiID {
			vdiListed = true
		}
	}
	if !vdiListed {
		t.Fatalf("the created VDI %s is not in 'vdi list'", vdiID)
	}

	// Step 3: attach it to the VM (create the VBD).
	var vbdOut map[string]any
	runJSON(t, run, &vbdOut, "vbd", "create", "--vm", vmID, "--vdi", vdiID, "--output", "json")
	vbdID, _ := vbdOut["vbd"].(string)
	if vbdID == "" {
		t.Fatalf("vbd create output has no vbd id: %s", vbdOut)
	}

	// Step 5: verify — the disk must be listed on the VM and the VBD
	// reference the right VM and VDI.
	var vmVdis []map[string]any
	runJSON(t, run, &vmVdis, "vm", "vdis", vmID, "--output", "json")
	vdisOnVM := false
	for _, v := range vmVdis {
		if v["id"] == vdiID {
			vdisOnVM = true
		}
	}
	if !vdisOnVM {
		t.Fatalf("'vm vdis' does not list the new VDI %s", vdiID)
	}

	var vbdList []map[string]any
	runJSON(t, run, &vbdList, "vbd", "list", "--vm", vmID, "--output", "json")
	vbdListed := false
	for _, v := range vbdList {
		if v["id"] == vbdID && v["VM"] == vmID && v["VDI"] == vdiID {
			vbdListed = true
		}
	}
	if !vbdListed {
		t.Fatalf("'vbd list --vm' does not show the new VBD %s (VM %s, VDI %s)", vbdID, vmID, vdiID)
	}

	// Step 4: the hot-plug / hot-unplug round trip. Both are asynchronous and
	// must return a task id.
	out, err := run("vbd", "connect", vbdID)
	if err != nil {
		t.Fatalf("xo vbd connect: %v\n%s", err, out)
	}
	if !strings.Contains(out, "(task ") {
		t.Fatalf("connect output should carry a task id:\n%s", out)
	}
	out, err = run("vbd", "disconnect", vbdID)
	if err != nil {
		t.Fatalf("xo vbd disconnect: %v\n%s", err, out)
	}
	if !strings.Contains(out, "(task ") {
		t.Fatalf("disconnect output should carry a task id:\n%s", out)
	}

	// Removing the disk, in reverse order: the VBD first (the VDI is kept),
	// then the VDI. Both are destructive: without --yes and a non-terminal
	// stdin they must refuse to run.
	if _, err := run("vbd", "delete", vbdID); err == nil {
		t.Fatalf("vbd delete without --yes must not proceed without a terminal")
	}
	if _, err := run("vdi", "delete", vdiID); err == nil {
		t.Fatalf("vdi delete without --yes must not proceed without a terminal")
	}
	if out, err := run("vbd", "delete", vbdID, "--yes"); err != nil {
		t.Fatalf("xo vbd delete --yes: %v\n%s", err, out)
	}
	if out, err := run("vdi", "delete", vdiID, "--yes"); err != nil {
		t.Fatalf("xo vdi delete --yes: %v\n%s", err, out)
	}

	// The VDI is gone.
	if out, err := run("vdi", "get", vdiID); err == nil {
		t.Fatalf("the deleted VDI must not be retrievable:\n%s", out)
	} else if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("get on a deleted VDI should report not found: %v", err)
	}
}

// TestIntegrationUseCaseImageToTemplate pins the "Turn a downloaded image
// into a VM template" use case (docs/usecases.md), the long (bare disk) path,
// with typed commands:
//
//  1. find the target pool and SR   xo pool list / the pool's default_SR
//     3b. create the shell VM          xo vm create (from a base template)
//     find the system disk          xo vbd list (the bootable VBD)
//     write the image into it       xo vdi import
//  4. boot it and verify            xo vm start --wait (power_state=Running)
//  5. shut it down cleanly          xo vm stop --wait  (power_state=Halted)
//  7. verify the template           xo template list
//  8. create VMs from it            xo vm create --template
//
// Step 3a (XVA/OVA) is not covered: the simulator answers the raw VDI
// streaming endpoints but not the VM archive import endpoint. Step 6 (VM ->
// template conversion) is done in the web UI: the REST API has no endpoint
// for it (a documented upstream gap), so it cannot be a CLI test. The guest
// verification in step 4 (console/SSH, "the OS really boots") is likewise
// out of reach: the test proves the power state transitions, not the OS.
func TestIntegrationUseCaseImageToTemplate(t *testing.T) {
	setupUseCaseEnv(t)
	run := newUseCaseRun(t)
	poolID, templateID, _ := pickTestEnvironment(t, run)

	// A non-empty raw image to import (a real guest OS is out of scope; the
	// content is irrelevant to the API, only that a byte stream is uploaded).
	// 1 MiB keeps the suite fast while still being a real upload.
	image := t.TempDir() + "/node.raw"
	if err := os.WriteFile(image, make([]byte, 1<<20), 0o600); err != nil {
		t.Fatalf("cannot write test image: %v", err)
	}

	// Step 3b.1: create the shell VM from a base template. It is created
	// halted, so its disk is safe to write.
	const name = "uc-node-base"
	var created map[string]any
	runJSON(t, run, &created, "vm", "create", name, "--pool", poolID, "--template", templateID, "--output", "json")
	vmID, _ := created["id"].(string)
	if vmID == "" {
		t.Fatalf("created shell VM has no id")
	}
	t.Cleanup(func() {
		if _, err := run("vm", "delete", vmID, "--yes"); err != nil {
			t.Logf("cleanup: xo vm delete %s: %v", vmID, err)
		}
	})

	// Step 3b.2: find the system disk — the VDI behind the bootable VBD.
	var vbdList []map[string]any
	runJSON(t, run, &vbdList, "vbd", "list", "--vm", vmID, "--output", "json")
	sysVdi := ""
	for _, v := range vbdList {
		if v["bootable"] == true {
			if id, _ := v["VDI"].(string); id != "" {
				sysVdi = id
			}
		}
	}
	if sysVdi == "" {
		t.Skip("the shell VM has no bootable system disk")
	}

	// Step 3b.3: write the image into it. Destructive: without --yes and a
	// non-terminal stdin it must refuse to run.
	if _, err := run("vdi", "import", sysVdi, image, "--format", "raw"); err == nil {
		t.Fatalf("vdi import without --yes must not proceed without a terminal")
	}
	if out, err := run("vdi", "import", sysVdi, image, "--format", "raw", "--yes"); err != nil {
		t.Fatalf("xo vdi import --yes: %v\n%s", err, out)
	}

	// Step 4: boot it. --wait blocks until the start task completes; the
	// power state must then be Running.
	if out, err := run("vm", "start", vmID, "--wait"); err != nil {
		t.Fatalf("xo vm start --wait: %v\n%s", err, out)
	}
	var state string
	runJSON(t, run, &state, "vm", "get", vmID, "--output", "json", "--query", "power_state")
	if state != "Running" {
		t.Fatalf("power_state after start --wait = %q, want Running", state)
	}

	// Step 5: shut it down cleanly, halted, for the (web UI) conversion.
	if out, err := run("vm", "stop", vmID, "--wait"); err != nil {
		t.Fatalf("xo vm stop --wait: %v\n%s", err, out)
	}
	state = ""
	runJSON(t, run, &state, "vm", "get", vmID, "--output", "json", "--query", "power_state")
	if state != "Halted" {
		t.Fatalf("power_state after stop --wait = %q, want Halted", state)
	}

	// Step 6 (VM -> template) is the web UI: the REST API has no endpoint for
	// it, so the flow pauses here, as documented in the use case. The typed
	// steps that follow the conversion are exercised against the pool's
	// existing template instead.

	// Step 7: the template list must be queryable by its id.
	var tplList []map[string]any
	runJSON(t, run, &tplList, "template", "list", "--output", "json")
	if len(tplList) == 0 {
		t.Fatalf("'template list' is empty, but %s was created from a template", name)
	}

	// Step 8: create a VM from the template — the whole point of the use
	// case. The fleet VM is cleaned up by the test cleanup below.
	var node map[string]any
	runJSON(t, run, &node, "vm", "create", "uc-node-01", "--pool", poolID, "--template", templateID, "--output", "json")
	nodeID, _ := node["id"].(string)
	if nodeID == "" || nodeID == vmID {
		t.Fatalf("the VM created from the template is not a new VM: %s", nodeID)
	}
	t.Cleanup(func() {
		if _, err := run("vm", "delete", nodeID, "--yes"); err != nil {
			t.Logf("cleanup: xo vm delete %s: %v", nodeID, err)
		}
	})
}
