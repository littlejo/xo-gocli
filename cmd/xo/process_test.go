package main

// Process-level tests for the scripting contract (docs/road-to-v1.md, S1).
//
// Every other test in this repository drives the in-process command APIs, so
// the behaviors a script actually relies on — exit codes, the stdout/stderr
// split, the "Error:" prefix, Ctrl+C propagation down to the HTTP layer,
// profile resolution from the config file, and the exact JSON shape — were
// never verified. That is how the 'xo rest' flag-merge panic shipped through
// a fully green suite. These tests close the gap by running the compiled
// binary as a child process against a fake Xen Orchestra server.
//
// The suite builds the real binary once (go build, cached) and runs it with a
// hermetic environment: a throwaway config file and no ambient XOA_*
// variables, so the results do not depend on the developer's own xo
// configuration. No live Xen Orchestra is required.

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// fixtureVMs is the deterministic payload the fake server answers with. It
// doubles as the seed for the golden files in testdata/.
const fixtureVMs = `[
	{
		"id": "550e8400-e29b-41d4-a716-446655440001",
		"name_label": "web-01",
		"name_description": "frontend for the test fixture",
		"power_state": "Running",
		"memory": {"size": 2147483648},
		"CPUs": {"number": 2, "max": 4},
		"boot": {"order": "cd", "firmware": "bios"},
		"type": "vm",
		"tags": ["web", "fixture"],
		"mainIpAddress": "10.0.0.11",
		"$container": "aaaaaaaa-bbbb-cccc-dddd-000000000001"
	},
	{
		"id": "550e8400-e29b-41d4-a716-446655440002",
		"name_label": "db-01",
		"power_state": "Halted",
		"memory": {"size": 4294967296},
		"CPUs": {"number": 4},
		"boot": {"order": "c"},
		"type": "vm",
		"$container": "aaaaaaaa-bbbb-cccc-dddd-000000000002"
	}
]`

// firstVMID is the id of the first fixture VM, used by the golden get test.
const firstVMID = "550e8400-e29b-41d4-a716-446655440001"

// fakeToken is the authentication token the fake server accepts.
const fakeToken = "process-test-token"

// updateGoldens, when passed as -update, rewrites the golden files instead of
// comparing against them.
var updateGoldens = flag.Bool("update", false, "rewrite the golden files in testdata/ instead of comparing against them")

// fixtureVMObjects and fixtureVMByID are the parsed fixture, kept as raw
// JSON so the fake server can return an exact object without reformatting it.
var (
	fixtureVMObjects []json.RawMessage
	fixtureVMByID    = make(map[string]json.RawMessage)
)

func init() {
	if err := json.Unmarshal([]byte(fixtureVMs), &fixtureVMObjects); err != nil {
		panic("invalid fixtureVMs: " + err.Error())
	}
	for _, raw := range fixtureVMObjects {
		var probe struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		fixtureVMByID[probe.ID] = raw
	}
}

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// TestMain tears down the built binary once the suite finishes. The build
// itself is lazy (requireBinary) so that 'go test -list' does not need it.
func TestMain(m *testing.M) {
	code := m.Run()
	if binPath != "" {
		_ = os.Remove(binPath)
	}
	os.Exit(code)
}

func buildBinary() {
	// go test runs with the package directory as CWD, so "." is cmd/xo.
	out, err := os.CreateTemp("", "xo-process-test-*.bin")
	if err != nil {
		binErr = fmt.Errorf("cannot create binary path: %w", err)
		return
	}
	binPath = out.Name()
	_ = out.Close()

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	var log bytes.Buffer
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err := cmd.Run(); err != nil {
		binErr = fmt.Errorf("go build .: %v\n%s", err, log.String())
	}
}

func requireBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(buildBinary)
	if binErr != nil {
		t.Fatalf("cannot build the xo binary for process tests: %v", binErr)
	}
	return binPath
}

// fakeXO is a minimal Xen Orchestra REST endpoint. It enforces token
// authentication (the SDK sends the token as an authenticationToken cookie),
// serves the fixture VMs on /rest/v0/vms and /rest/v0/vms/<id>, can force a
// fixed non-2xx status, and in block mode holds the list request until the
// client goes away (used to observe context cancellation).
type fakeXO struct {
	server *httptest.Server

	// status, when non-zero, makes every request return that status.
	status int

	// block, when true, holds GET /rest/v0/vms until the request context is
	// cancelled.
	block bool

	requestSeen chan struct{} // closed once a (blocked) request arrives
	cancelSeen  chan struct{} // closed once the blocked request is cancelled
	blockOnce   sync.Once
}

func newFakeXO(t *testing.T, status int, block bool) *fakeXO {
	t.Helper()
	fx := &fakeXO{
		status:      status,
		block:       block,
		requestSeen: make(chan struct{}),
		cancelSeen:  make(chan struct{}),
	}
	fx.server = httptest.NewServer(http.HandlerFunc(fx.handle))
	t.Cleanup(fx.server.Close)
	return fx
}

func (fx *fakeXO) URL() string { return fx.server.URL }

func (fx *fakeXO) handle(w http.ResponseWriter, r *http.Request) {
	if fx.block {
		fx.blockOnce.Do(func() {
			close(fx.requestSeen)
			go func() {
				<-r.Context().Done()
				close(fx.cancelSeen)
			}()
		})
		<-r.Context().Done()
		return
	}

	if fx.status != 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(fx.status)
		_, _ = fmt.Fprintf(w, `{"message":"injected %d"}`, fx.status)
		return
	}

	if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != fakeToken {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
		return
	}

	switch {
	case r.URL.Path == "/rest/v0/vms":
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureVMs))
	case strings.HasPrefix(r.URL.Path, "/rest/v0/vms/"):
		id := strings.TrimPrefix(r.URL.Path, "/rest/v0/vms/")
		if raw, ok := fixtureVMByID[id]; ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{"message":"object %s not found"}`, id)
	default:
		http.NotFound(w, r)
	}
}

// runResult captures a child-process run.
type runResult struct {
	stdout string
	stderr string
	code   int
}

// cleanEnv returns an environment for the child process: the current
// environment minus every XOA_* variable (so the developer's xo setup cannot
// leak in), plus the XOA_* variables the test wants to set.
func cleanEnv(extra ...string) []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "XOA_") {
			continue
		}
		env = append(env, e)
	}
	return append(env, extra...)
}

// run executes the binary with the given environment and arguments, waiting
// for it to exit on its own.
func run(t *testing.T, bin string, env []string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return runResult{
		stdout: out.String(),
		stderr: errb.String(),
		code:   exitCode(err),
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	// Could not start the process at all — a hard failure marker.
	return -1
}

// envFor returns the child environment pointing at a fake server, with a
// throwaway (empty) config file so the endpoint and token come only from the
// environment.
func envFor(t *testing.T, endpoint, token string) []string {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config")
	return cleanEnv(
		"XOA_CONFIG_FILE="+cfg,
		"XOA_ENDPOINT="+endpoint,
		"XOA_TOKEN="+token,
	)
}

func TestMainSmokeHelp(t *testing.T) {
	// Even 'xo --help' and every resource subcommand '--help' must work
	// offline: this is the class of regression (the 'rest' flag-merge panic)
	// that in-process tests never caught.
	bin := requireBinary(t)
	groups := []string{"", "vm", "host", "pool", "sr", "network", "template", "task", "vdi", "vbd", "pbd", "token", "rest", "configure"}
	for _, group := range groups {
		args := []string{}
		if group != "" {
			args = append(args, group)
		}
		args = append(args, "--help")
		res := run(t, bin, cleanEnv("XOA_CONFIG_FILE="+filepath.Join(t.TempDir(), "config")), args...)
		if res.code != 0 {
			t.Fatalf("%q --help: exit %d, stderr:\n%s", group, res.code, res.stderr)
		}
		if strings.Contains(res.stderr, "panic") {
			t.Fatalf("%q --help: panic on stderr:\n%s", group, res.stderr)
		}
	}
}

func TestExitCodesAndStderrDiscipline(t *testing.T) {
	bin := requireBinary(t)

	// Success: exit 0, data on stdout, nothing on stderr.
	ok := newFakeXO(t, 0, false)
	res := run(t, bin, envFor(t, ok.URL(), fakeToken), "vm", "list")
	if res.code != 0 {
		t.Fatalf("success: exit %d, stderr:\n%s", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "web-01") {
		t.Fatalf("success: expected web-01 on stdout, got:\n%s", res.stdout)
	}
	if res.stderr != "" {
		t.Fatalf("success: expected empty stderr, got:\n%s", res.stderr)
	}

	// API error (500): exit 1, "Error:" on stderr, nothing on stdout.
	apiErr := newFakeXO(t, http.StatusInternalServerError, false)
	res = run(t, bin, envFor(t, apiErr.URL(), fakeToken), "vm", "list")
	if res.code != 1 {
		t.Fatalf("api error: exit %d (want 1), stderr:\n%s", res.code, res.stderr)
	}
	if !strings.HasPrefix(res.stderr, "Error: ") {
		t.Fatalf("api error: stderr should start with 'Error: ', got:\n%s", res.stderr)
	}
	if !strings.Contains(res.stderr, "500") {
		t.Fatalf("api error: stderr should mention the status, got:\n%s", res.stderr)
	}
	if res.stdout != "" {
		t.Fatalf("api error: expected empty stdout, got:\n%s", res.stdout)
	}

	// Usage error (missing required arg): exit 1, "Error:" on stderr, nothing
	// on stdout.
	res = run(t, bin, envFor(t, ok.URL(), fakeToken), "vm", "get")
	if res.code != 1 {
		t.Fatalf("usage error: exit %d (want 1), stderr:\n%s", res.code, res.stderr)
	}
	if !strings.HasPrefix(res.stderr, "Error: ") {
		t.Fatalf("usage error: stderr should start with 'Error: ', got:\n%s", res.stderr)
	}
	if res.stdout != "" {
		t.Fatalf("usage error: expected empty stdout, got:\n%s", res.stdout)
	}

	// Unknown flag: exit 1, "Error:" on stderr, nothing on stdout.
	res = run(t, bin, envFor(t, ok.URL(), fakeToken), "vm", "list", "--nope")
	if res.code != 1 {
		t.Fatalf("unknown flag: exit %d (want 1), stderr:\n%s", res.code, res.stderr)
	}
	if !strings.HasPrefix(res.stderr, "Error: ") {
		t.Fatalf("unknown flag: stderr should start with 'Error: ', got:\n%s", res.stderr)
	}
	if res.stdout != "" {
		t.Fatalf("unknown flag: expected empty stdout, got:\n%s", res.stdout)
	}
}

func TestRejectedToken401(t *testing.T) {
	bin := requireBinary(t)
	ok := newFakeXO(t, 0, false)

	// A wrong token is the most common real-world failure: the server must
	// reject it with 401 and the CLI must surface a clean error — and never
	// leak the secret in the message.
	const wrongToken = "definitely-not-the-token"
	res := run(t, bin, envFor(t, ok.URL(), wrongToken), "vm", "list")
	if res.code != 1 {
		t.Fatalf("401: exit %d (want 1), stderr:\n%s", res.code, res.stderr)
	}
	if !strings.HasPrefix(res.stderr, "Error: ") {
		t.Fatalf("401: stderr should start with 'Error: ', got:\n%s", res.stderr)
	}
	if !strings.Contains(res.stderr, "401") {
		t.Fatalf("401: stderr should mention 401, got:\n%s", res.stderr)
	}
	if res.stdout != "" {
		t.Fatalf("401: expected empty stdout, got:\n%s", res.stdout)
	}
	if strings.Contains(res.stderr, wrongToken) {
		t.Fatalf("401: the rejected token leaked into stderr:\n%s", res.stderr)
	}
}

func TestContextCancellationPropagates(t *testing.T) {
	bin := requireBinary(t)
	fx := newFakeXO(t, 0, true)

	cmd := exec.Command(bin, "vm", "list", "--output", "json")
	cmd.Env = envFor(t, fx.URL(), fakeToken)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	startedAt := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatalf("cannot start xo: %v", err)
	}

	// A single goroutine owns cmd.Wait so the process is reaped exactly once.
	// The buffered channel means it can never block, and the test body only
	// receives its result (it never calls Wait itself).
	exitCh := make(chan error, 1)
	go func() { exitCh <- cmd.Wait() }()
	t.Cleanup(func() {
		// Best-effort: if an assertion failed before the process exited, do
		// not leave it running. Kill only signals the (immutable) pid, so it
		// is safe even while the reaper above is still waiting on it.
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	// Wait until the request has actually reached the (blocking) server.
	select {
	case <-fx.requestSeen:
	case <-time.After(10 * time.Second):
		t.Fatal("the request never reached the fake server")
	}

	// Ctrl+C: the cancellation must reach the HTTP layer and abort the
	// in-flight request, so the process exits well before the 30 s client
	// timeout.
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("cannot signal xo: %v", err)
	}

	// The server must observe the request context being cancelled — proof the
	// Ctrl+C → command → SDK → HTTP chain works.
	select {
	case <-fx.cancelSeen:
	case <-time.After(10 * time.Second):
		t.Fatal("the fake server never saw the request context cancel")
	}

	// The process must have exited.
	var exitErr error
	select {
	case exitErr = <-exitCh:
	case <-time.After(10 * time.Second):
		t.Fatal("xo did not exit after Ctrl+C")
	}

	elapsed := time.Since(startedAt)
	if elapsed > 10*time.Second {
		t.Fatalf("xo took %v to exit after Ctrl+C (should be well under the 30 s timeout)", elapsed)
	}
	if exitCode(exitErr) == 0 {
		t.Fatalf("ctrl+c: expected a non-zero exit code, stderr:\n%s", errb.String())
	}
	if out.Len() != 0 {
		t.Fatalf("ctrl+c: expected empty stdout, got:\n%s", out.String())
	}
	if !strings.HasPrefix(errb.String(), "Error: ") {
		t.Fatalf("ctrl+c: stderr should start with 'Error: ', got:\n%s", errb.String())
	}
}

func TestProfileEndToEnd(t *testing.T) {
	bin := requireBinary(t)
	ok := newFakeXO(t, 0, false)

	// A profile seeded in the config file, selected with --profile and with no
	// XOA_ENDPOINT / XOA_TOKEN in the environment at all.
	cfgFile := filepath.Join(t.TempDir(), "config")
	cfg := fmt.Sprintf("current: lab\nprofiles:\n  - name: lab\n    endpoint: %s\n    token: %s\n", ok.URL(), fakeToken)
	if err := os.WriteFile(cfgFile, []byte(cfg), 0o600); err != nil {
		t.Fatalf("cannot write config: %v", err)
	}

	res := run(t, bin, cleanEnv("XOA_CONFIG_FILE="+cfgFile), "vm", "list", "--profile", "lab", "--output", "json")
	if res.code != 0 {
		t.Fatalf("profile: exit %d, stderr:\n%s", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "web-01") {
		t.Fatalf("profile: expected web-01 on stdout, got:\n%s", res.stdout)
	}
	if res.stderr != "" {
		t.Fatalf("profile: expected empty stderr, got:\n%s", res.stderr)
	}

	// An unknown profile is a clean usage error.
	res = run(t, bin, cleanEnv("XOA_CONFIG_FILE="+cfgFile), "vm", "list", "--profile", "does-not-exist")
	if res.code != 1 {
		t.Fatalf("unknown profile: exit %d (want 1), stderr:\n%s", res.code, res.stderr)
	}
	if !strings.HasPrefix(res.stderr, "Error: ") {
		t.Fatalf("unknown profile: stderr should start with 'Error: ', got:\n%s", res.stderr)
	}
	if !strings.Contains(res.stderr, "not configured") {
		t.Fatalf("unknown profile: expected a 'not configured' message, got:\n%s", res.stderr)
	}
	if res.stdout != "" {
		t.Fatalf("unknown profile: expected empty stdout, got:\n%s", res.stdout)
	}
}

// golden runs the binary and either compares stdout to the named golden file
// (normal) or rewrites it (with -update).
func golden(t *testing.T, bin, goldenFile string, env []string, args ...string) {
	t.Helper()
	res := run(t, bin, env, args...)
	if res.code != 0 {
		t.Fatalf("%v: exit %d, stderr:\n%s", args, res.code, res.stderr)
	}
	got := strings.TrimRight(res.stdout, "\n")

	path := filepath.Join("testdata", goldenFile)
	if *updateGoldens {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("cannot create testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o600); err != nil {
			t.Fatalf("cannot write golden: %v", err)
		}
		t.Logf("updated golden %s", path)
		return
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read golden %s: %v (run with -update to create it)", path, err)
	}
	want := strings.TrimRight(string(wantBytes), "\n")
	if got != want {
		t.Errorf("%v: JSON output does not match golden %s\n--- got ---\n%s\n--- want ---\n%s", args, path, got, want)
	}
}

func TestGoldenJSONList(t *testing.T) {
	bin := requireBinary(t)
	ok := newFakeXO(t, 0, false)
	golden(t, bin, "vm-list.json", envFor(t, ok.URL(), fakeToken), "vm", "list", "--output", "json")
}

func TestGoldenJSONGet(t *testing.T) {
	bin := requireBinary(t)
	ok := newFakeXO(t, 0, false)
	golden(t, bin, "vm-get.json", envFor(t, ok.URL(), fakeToken), "vm", "get", firstVMID, "--output", "json")
}
