package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	xoconfig "github.com/littlejo/xo-gocli/internal/config"
)

// fakeXOVMS serves the VM list endpoint behind the self-signed TLS
// certificate of httptest.NewTLSServer.
func fakeXOVMS(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/vms" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"550e8400-e29b-41d4-a716-446655440001","name_label":"web-01","power_state":"Running","memory":{"size":1024},"CPUs":{"number":1},"boot":{},"type":"vm"}]`))
	}))
}

func newInsecureTestConfig(url string, insecure bool) *xoconfig.ClientConfig {
	return &xoconfig.ClientConfig{Endpoint: url, Token: "t", Insecure: insecure}
}

func TestNewClientRejectsSelfSignedWithoutInsecure(t *testing.T) {
	server := fakeXOVMS(t)
	defer server.Close()

	client, err := NewClient(nil, newInsecureTestConfig(server.URL, false))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.VM().GetAll(context.Background(), 0, ""); err == nil {
		t.Fatal("expected a TLS verification error")
	}
}

func TestNewClientAcceptsSelfSignedWithInsecure(t *testing.T) {
	server := fakeXOVMS(t)
	defer server.Close()

	client, err := NewClient(nil, newInsecureTestConfig(server.URL, true))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	vms, err := client.VM().GetAll(context.Background(), 0, "")
	if err != nil {
		t.Fatalf("GetAll with Insecure: %v", err)
	}
	if len(vms) != 1 || vms[0].NameLabel != "web-01" {
		t.Fatalf("unexpected VMs: %+v", vms)
	}
}

func TestWithInsecureHint(t *testing.T) {
	if err := InsecureHint("certificate is not valid for any names", false); err == nil ||
		!strings.Contains(err.Error(), "--insecure") {
		t.Errorf("expected an insecure hint, got: %v", err)
	}
	if err := InsecureHint("certificate is not valid for any names", true); err == nil ||
		strings.Contains(err.Error(), "--insecure") {
		t.Errorf("no hint should be added when insecure is already on: %v", err)
	}
	if err := InsecureHint("connection refused", false); err == nil ||
		strings.Contains(err.Error(), "--insecure") {
		t.Errorf("no hint should be added for non-TLS errors: %v", err)
	}
}

func newDebugTestRoot(fail error) *cobra.Command {
	root := &cobra.Command{Use: "xo", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().BoolP(FlagDebug, "d", false, "")
	root.AddCommand(&cobra.Command{
		Use:  "fail",
		RunE: func(*cobra.Command, []string) error { return fail },
	})
	return root
}

// isolateDebugEnv points the config resolution at an empty file with fixed
// credentials so the debug profile line is deterministic.
func isolateDebugEnv(t *testing.T) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE", "XOA_YES"} {
		t.Setenv(key, "")
	}
	t.Setenv("XOA_ENDPOINT", "https://xoa.test")
	t.Setenv("XOA_TOKEN", "test-token")
}

func TestDebugFromFlag(t *testing.T) {
	// The flag value is only populated once cobra parses it, i.e. inside
	// Execute, so the test must go through it before asking Debug.
	root := newDebugTestRoot(nil)
	if err := Execute(context.Background(), root, []string{"fail", "--debug"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !Debug(root) {
		t.Fatal("--debug must enable debug mode")
	}
}

func TestDebugFromEnvVar(t *testing.T) {
	for _, value := range []string{"1", "true", "yes", "TRUE"} {
		t.Setenv(EnvDebug, value)
		root := newDebugTestRoot(nil)
		if !Debug(root) {
			t.Fatalf("$%s=%q must enable debug mode", EnvDebug, value)
		}
	}
}

func TestDebugOffByDefault(t *testing.T) {
	t.Setenv(EnvDebug, "")
	root := newDebugTestRoot(nil)
	if Debug(root) {
		t.Fatal("debug mode must be off by default")
	}
}

func TestNotFound404IsConciseButCarriesDetail(t *testing.T) {
	err := NotFound("host", "get", "abc", errors.New("API error: 404 Not Found - not found"), false)
	if !strings.Contains(err.Error(), `host "abc" not found`) {
		t.Fatalf("expected a concise not-found message: %v", err)
	}
	if strings.Contains(err.Error(), "API error") {
		t.Fatalf("the raw API error must not leak into the normal message: %v", err)
	}
	if d := Detail(err); !strings.Contains(d, "404 Not Found") {
		t.Fatalf("the debug detail must carry the raw API error, got: %q", d)
	}
}

func TestNotFoundOtherErrorKeepsFullMessage(t *testing.T) {
	err := NotFound("VM", "resolve", "abc", errors.New("x509: certificate signed by unknown authority"), false)
	if !strings.Contains(err.Error(), `cannot resolve VM "abc"`) {
		t.Fatalf("expected the cannot-resolve form: %v", err)
	}
	if Detail(err) != "" {
		t.Fatalf("non-404 errors must not carry a hidden detail: %q", Detail(err))
	}
}

// --- timeout ----------------------------------------------------------------

// newTimeoutTestRoot builds a minimal root carrying the global --timeout flag;
// the noop subcommand records itself so callers can read the parsed flag value
// on a real cobra command (the value is only populated once cobra parses).
func newTimeoutTestRoot(captured **cobra.Command) *cobra.Command {
	root := &cobra.Command{Use: "xo", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().Duration(FlagTimeout, 0, "")
	root.AddCommand(&cobra.Command{
		Use:  "noop",
		RunE: func(cmd *cobra.Command, _ []string) error { *captured = cmd; return nil },
	})
	return root
}

func TestTimeoutDefault(t *testing.T) {
	t.Setenv(EnvTimeout, "")
	var captured *cobra.Command
	root := newTimeoutTestRoot(&captured)
	if got := Timeout(root); got != defaultClientTimeout {
		t.Fatalf("Timeout = %v, want the %v default", got, defaultClientTimeout)
	}
	if got := Timeout(nil); got != defaultClientTimeout {
		t.Fatalf("Timeout(nil) = %v, want the %v default", got, defaultClientTimeout)
	}
}

// isolateOutputEnv points the format resolution at an empty config file and
// clears the variables the chain reads.
func isolateOutputEnv(t *testing.T) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{xoconfig.EnvProfile, xoconfig.EnvEndpoint, xoconfig.EnvToken, xoconfig.EnvUsername, xoconfig.EnvPassword, xoconfig.EnvInsecure, xoconfig.EnvDefaultOutput} {
		t.Setenv(key, "")
	}
}

func newOutputTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "xo", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringP(FlagOutput, "o", "table", "")
	root.PersistentFlags().Bool(FlagJSON, false, "")
	root.AddCommand(&cobra.Command{
		Use:  "noop",
		RunE: func(*cobra.Command, []string) error { return nil },
	})
	return root
}

func TestOutputFormatDefault(t *testing.T) {
	isolateOutputEnv(t)
	root := newOutputTestRoot()
	if err := Execute(context.Background(), root, []string{"noop"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := OutputFormat(root); got != "table" {
		t.Fatalf("OutputFormat = %q, want table by default", got)
	}
	if got := OutputFormat(nil); got != "table" {
		t.Fatalf("OutputFormat(nil) = %q, want table", got)
	}
}

func TestOutputFormatFromFlag(t *testing.T) {
	isolateOutputEnv(t)
	root := newOutputTestRoot()
	if err := Execute(context.Background(), root, []string{"noop", "--output", "yaml"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := OutputFormat(root); got != "yaml" {
		t.Fatalf("--output yaml = %q", got)
	}
}

func TestOutputFormatFromJSONFlag(t *testing.T) {
	isolateOutputEnv(t)
	root := newOutputTestRoot()
	if err := Execute(context.Background(), root, []string{"noop", "--json"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := OutputFormat(root); got != "json" {
		t.Fatalf("--json = %q, want json", got)
	}
}

func TestOutputFormatFlagWinsOverJSONFlag(t *testing.T) {
	isolateOutputEnv(t)
	root := newOutputTestRoot()
	if err := Execute(context.Background(), root, []string{"noop", "--output", "text", "--json"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := OutputFormat(root); got != "text" {
		t.Fatalf("--output text --json = %q, want text", got)
	}
}

func TestOutputFormatFromEnvVar(t *testing.T) {
	isolateOutputEnv(t)
	t.Setenv(xoconfig.EnvDefaultOutput, "json")
	root := newOutputTestRoot()
	if err := Execute(context.Background(), root, []string{"noop"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := OutputFormat(root); got != "json" {
		t.Fatalf("$%s=json = %q", xoconfig.EnvDefaultOutput, got)
	}
}

func TestOutputFormatEnvVarBeatsProfileOutput(t *testing.T) {
	isolateOutputEnv(t)
	if _, err := xoconfig.Upsert(xoconfig.Profile{Name: "lab", Endpoint: "https://lab.test", Token: "t", Output: "yaml"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	t.Setenv(xoconfig.EnvDefaultOutput, "json")
	root := newOutputTestRoot()
	if err := Execute(context.Background(), root, []string{"noop"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := OutputFormat(root); got != "json" {
		t.Fatalf("$%s must beat the profile output: %q", xoconfig.EnvDefaultOutput, got)
	}
}

func TestOutputFormatFromProfile(t *testing.T) {
	isolateOutputEnv(t)
	if _, err := xoconfig.Upsert(xoconfig.Profile{Name: "lab", Endpoint: "https://lab.test", Token: "t", Output: "yaml"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	root := newOutputTestRoot()
	if err := Execute(context.Background(), root, []string{"noop"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := OutputFormat(root); got != "yaml" {
		t.Fatalf("profile output = %q, want yaml", got)
	}
}

func TestOutputFormatUnsetFlagFallsBackToEnvVar(t *testing.T) {
	// The --output flag is registered with the default value "table", so an
	// invocation that does not pass it must still fall through to the
	// environment variable and the profile.
	isolateOutputEnv(t)
	t.Setenv(xoconfig.EnvDefaultOutput, "yaml")
	root := newOutputTestRoot()
	if err := Execute(context.Background(), root, []string{"noop"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := OutputFormat(root); got != "yaml" {
		t.Fatalf("unset --output must fall back to $%s: %q", xoconfig.EnvDefaultOutput, got)
	}
}

func TestOutputFormatFlagWinsOverEnvVar(t *testing.T) {
	isolateOutputEnv(t)
	t.Setenv(xoconfig.EnvDefaultOutput, "json")
	root := newOutputTestRoot()
	if err := Execute(context.Background(), root, []string{"noop", "--output", "text"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := OutputFormat(root); got != "text" {
		t.Fatalf("--output must beat $%s: %q", xoconfig.EnvDefaultOutput, got)
	}
}

func TestTimeoutFromFlag(t *testing.T) {
	t.Setenv(EnvTimeout, "")
	var captured *cobra.Command
	root := newTimeoutTestRoot(&captured)
	if err := Execute(context.Background(), root, []string{"noop", "--timeout", "90s"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := Timeout(root); got != 90*time.Second {
		t.Fatalf("--timeout 90s = %v, want 90s", got)
	}
}

func TestTimeoutFromEnvVar(t *testing.T) {
	t.Setenv(EnvTimeout, "2m")
	var captured *cobra.Command
	root := newTimeoutTestRoot(&captured)
	if got := Timeout(root); got != 2*time.Minute {
		t.Fatalf("$%s=2m = %v, want 2m", EnvTimeout, got)
	}
}

func TestTimeoutFlagWinsOverEnvVar(t *testing.T) {
	t.Setenv(EnvTimeout, "2m")
	var captured *cobra.Command
	root := newTimeoutTestRoot(&captured)
	if err := Execute(context.Background(), root, []string{"noop", "--timeout", "90s"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := Timeout(root); got != 90*time.Second {
		t.Fatalf("--timeout must win over $%s, got %v", EnvTimeout, got)
	}
}

func TestTimeoutInvalidEnvVarFallsBackToDefault(t *testing.T) {
	t.Setenv(EnvTimeout, "not-a-duration")
	var captured *cobra.Command
	root := newTimeoutTestRoot(&captured)
	if got := Timeout(root); got != defaultClientTimeout {
		t.Fatalf("invalid $%s must fall back to the default, got %v", EnvTimeout, got)
	}
}

// buildSDKConfig must hand the requested duration to the SDK (its HTTP client
// timeout), and keep the SDK's 30-second default when given zero.
func TestBuildSDKConfigPassesTimeout(t *testing.T) {
	cfg := &xoconfig.ClientConfig{Endpoint: "https://xoa.test", Token: "t"}

	sdk, err := buildSDKConfig(cfg, 2*time.Minute)
	if err != nil {
		t.Fatalf("buildSDKConfig: %v", err)
	}
	if sdk.ClientTimeout != 2*time.Minute {
		t.Fatalf("ClientTimeout = %v, want 2m", sdk.ClientTimeout)
	}

	sdk, err = buildSDKConfig(cfg, 0)
	if err != nil {
		t.Fatalf("buildSDKConfig: %v", err)
	}
	if sdk.ClientTimeout != 30*time.Second {
		t.Fatalf("ClientTimeout = %v, want the 30s SDK default", sdk.ClientTimeout)
	}
}

// fakeSlowXO answers /rest/v0/vms only after sleeping, so a client timeout
// shorter than the sleep fails and a longer one succeeds.
func fakeSlowXO(t *testing.T, sleep time.Duration) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(sleep)
		if r.URL.Path != "/rest/v0/vms" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
}

// TestNewClientAppliesTimeout proves the --timeout value actually reaches the
// SDK's HTTP client end to end: the same 300-millisecond response succeeds
// under the 30-second default but is cut short by --timeout 50ms.
func TestNewClientAppliesTimeout(t *testing.T) {
	t.Setenv(EnvTimeout, "")
	server := fakeSlowXO(t, 300*time.Millisecond)
	defer server.Close()
	cfg := newInsecureTestConfig(server.URL, true)

	client, err := NewClient(nil, cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.VM().GetAll(context.Background(), 0, ""); err != nil {
		t.Fatalf("GetAll under the default timeout: %v", err)
	}

	var captured *cobra.Command
	root := newTimeoutTestRoot(&captured)
	if err := Execute(context.Background(), root, []string{"noop", "--timeout", "50ms"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	client, err = NewClient(captured, cfg)
	if err != nil {
		t.Fatalf("NewClient with --timeout: %v", err)
	}
	if _, err := client.VM().GetAll(context.Background(), 0, ""); err == nil {
		t.Fatal("expected a timeout error with --timeout 50ms")
	} else if !strings.Contains(strings.ToLower(err.Error()), "timeout") {
		t.Fatalf("expected a timeout error, got: %v", err)
	}
}

func TestExecutePrintsDebugDetails(t *testing.T) {
	isolateDebugEnv(t)
	t.Setenv(EnvDebug, "")
	raw := errors.New("API error: 404 Not Found - not found")
	root := newDebugTestRoot(NotFound("host", "get", "abc", raw, false))

	var errOut strings.Builder
	root.SetErr(&errOut)
	if err := Execute(context.Background(), root, []string{"fail", "--debug"}); err == nil {
		t.Fatal("expected the injected error")
	}
	got := errOut.String()
	if !strings.Contains(got, "debug: profile=default endpoint=https://xoa.test") {
		t.Errorf("debug output must name the resolved profile and endpoint:\n%s", got)
	}
	if !strings.Contains(got, "404 Not Found") {
		t.Errorf("debug output must reveal the raw API error:\n%s", got)
	}
}

func TestExecutePrintsNothingWithoutDebug(t *testing.T) {
	isolateDebugEnv(t)
	t.Setenv(EnvDebug, "")
	raw := errors.New("API error: 404 Not Found - not found")
	root := newDebugTestRoot(NotFound("host", "get", "abc", raw, false))

	var errOut strings.Builder
	root.SetErr(&errOut)
	if err := Execute(context.Background(), root, []string{"fail"}); err == nil {
		t.Fatal("expected the injected error")
	}
	if errOut.String() != "" {
		t.Errorf("no diagnostics expected without --debug, got:\n%s", errOut.String())
	}
}

func TestExecuteDebugViaEnvVar(t *testing.T) {
	isolateDebugEnv(t)
	t.Setenv(EnvDebug, "1")
	raw := errors.New("API error: 404 Not Found - not found")
	root := newDebugTestRoot(NotFound("host", "get", "abc", raw, false))

	var errOut strings.Builder
	root.SetErr(&errOut)
	if err := Execute(context.Background(), root, []string{"fail"}); err == nil {
		t.Fatal("expected the injected error")
	}
	if !strings.Contains(errOut.String(), "404 Not Found") {
		t.Errorf("$XOA_DEBUG must reveal the raw API error:\n%s", errOut.String())
	}
}
