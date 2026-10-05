package configure

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
)

// newTestRoot mirrors the production root flags that configure relies on.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	root.AddCommand(NewCommand())
	return root
}

func runConfigure(t *testing.T, args ...string) string {
	t.Helper()
	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(append([]string{"configure"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("configure %v: %v (output: %s)", args, err, out.String())
	}
	return out.String()
}

func runConfigureExpectError(t *testing.T, args ...string) string {
	t.Helper()
	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(append([]string{"configure"}, args...))
	err := root.Execute()
	if err == nil {
		t.Fatalf("configure %v: expected an error", args)
	}
	return err.Error()
}

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE"} {
		t.Setenv(key, "")
	}
}

func TestConfigureNonInteractiveSavesProfile(t *testing.T) {
	isolate(t)

	runConfigure(t, "--profile", "lab", "--endpoint", "https://xo.lab.example.com", "--token", "secret")

	cfg, err := config.Load("lab")
	if err != nil {
		t.Fatalf("Load after configure: %v", err)
	}
	if cfg.Endpoint != "https://xo.lab.example.com" || cfg.Token != "secret" {
		t.Fatalf("unexpected stored profile: %+v", cfg)
	}
}

func TestConfigureDefaultProfile(t *testing.T) {
	isolate(t)

	runConfigure(t, "--endpoint", "https://xo.example.com", "--token", "secret")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load after configure: %v", err)
	}
	if cfg.Name != config.DefaultProfile || cfg.Endpoint != "https://xo.example.com" {
		t.Fatalf("unexpected stored profile: %+v", cfg)
	}
}

func TestConfigureRejectsBadEndpoint(t *testing.T) {
	isolate(t)

	out := runConfigureExpectError(t, "--endpoint", "ftp://nope", "--token", "x")
	if !strings.Contains(out, "http") {
		t.Fatalf("unexpected error output: %s", out)
	}
}

func TestConfigureNonInteractiveRequiresCredentials(t *testing.T) {
	isolate(t)

	out := runConfigureExpectError(t, "--endpoint", "https://xo.example.com")
	if !strings.Contains(out, "token") && !strings.Contains(out, "username") {
		t.Fatalf("unexpected error output: %s", out)
	}
}

func TestConfigureUsernamePassword(t *testing.T) {
	isolate(t)

	runConfigure(t, "--endpoint", "https://xo.example.com", "--username", "admin", "--password", "s3cret")

	cfg, err := config.Load(config.DefaultProfile)
	if err != nil {
		t.Fatalf("Load after configure: %v", err)
	}
	if cfg.Username != "admin" || cfg.Password != "s3cret" || cfg.Token != "" {
		t.Fatalf("unexpected stored profile: %+v", cfg)
	}
}

// XOA_ENDPOINT seeds the endpoint when --endpoint is not given, exactly like
// XOA_TOKEN/XOA_USERNAME/XOA_PASSWORD seed their fields.

func TestConfigureEndpointFromEnvironment(t *testing.T) {
	isolate(t)
	t.Setenv(config.EnvEndpoint, "https://from-env.example.com")

	runConfigure(t, "--token", "secret")

	cfg, err := config.Load(config.DefaultProfile)
	if err != nil {
		t.Fatalf("Load after configure: %v", err)
	}
	if cfg.Endpoint != "https://from-env.example.com" {
		t.Fatalf("XOA_ENDPOINT was not used as the endpoint: %+v", cfg)
	}
}

func TestConfigureEndpointFlagBeatsEnvironment(t *testing.T) {
	isolate(t)
	t.Setenv(config.EnvEndpoint, "https://from-env.example.com")

	runConfigure(t, "--endpoint", "https://from-flag.example.com", "--token", "secret")

	// The stored profile must carry the flag value. Load would re-apply the
	// XOA_ENDPOINT runtime override, so clear it before checking the file.
	t.Setenv(config.EnvEndpoint, "")
	cfg, err := config.Load(config.DefaultProfile)
	if err != nil {
		t.Fatalf("Load after configure: %v", err)
	}
	if cfg.Endpoint != "https://from-flag.example.com" {
		t.Fatalf("--endpoint must win over XOA_ENDPOINT when storing: %+v", cfg)
	}
}
