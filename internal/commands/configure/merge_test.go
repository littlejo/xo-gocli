package configure

import (
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/config"
)

func storedProfile(t *testing.T, name string) config.Profile {
	t.Helper()
	p, err := config.Lookup(name)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatalf("profile %q is not stored", name)
	}
	return *p
}

func currentProfile(t *testing.T) string {
	t.Helper()
	current, _, err := config.List()
	if err != nil {
		t.Fatal(err)
	}
	return current
}

// An update only changes what is given: the endpoint, --insecure and the
// "output" setting survive a token rotation.
func TestConfigureUpdateKeepsUnchangedValues(t *testing.T) {
	isolate(t)
	runConfigure(t, "--profile", "lab", "--endpoint", "https://lab.example", "--token", "old", "--insecure")
	p := storedProfile(t, "lab")
	p.Output = "json"
	if _, err := config.Upsert(p, false); err != nil {
		t.Fatal(err)
	}

	runConfigure(t, "--profile", "lab", "--token", "new")

	got := storedProfile(t, "lab")
	if got.Endpoint != "https://lab.example" || got.Token != "new" || !got.Insecure || got.Output != "json" {
		t.Fatalf("after a token update: %+v", got)
	}

	runConfigure(t, "--profile", "lab", "--insecure=false")
	if got := storedProfile(t, "lab"); got.Insecure || got.Token != "new" {
		t.Fatalf("--insecure=false should only clear insecure: %+v", got)
	}
}

// Giving a token replaces stored password credentials, and the reverse.
func TestConfigureSwitchesCredentials(t *testing.T) {
	isolate(t)
	runConfigure(t, "--endpoint", "https://xo.example", "--username", "admin", "--password", "secret")
	runConfigure(t, "--token", "tok")
	if got := storedProfile(t, "default"); got.Token != "tok" || got.Username != "" || got.Password != "" {
		t.Fatalf("token should replace the password: %+v", got)
	}
	runConfigure(t, "--username", "admin", "--password", "secret")
	if got := storedProfile(t, "default"); got.Token != "" || got.Username != "admin" || got.Password != "secret" {
		t.Fatalf("password should replace the token: %+v", got)
	}
}

// Only the first stored profile becomes current; 'use' switches explicitly.
func TestConfigureCurrentProfileIsExplicit(t *testing.T) {
	isolate(t)
	runConfigure(t, "--profile", "prod", "--endpoint", "https://prod.example", "--token", "p")
	if got := currentProfile(t); got != "prod" {
		t.Fatalf("the first profile should become current, got %q", got)
	}

	out := runConfigure(t, "--profile", "lab", "--endpoint", "https://lab.example", "--token", "l")
	if got := currentProfile(t); got != "prod" {
		t.Fatalf("storing another profile must not switch the current one, got %q", got)
	}
	if !strings.Contains(out, "xo configure use lab") {
		t.Fatalf("expected a hint to switch, got:\n%s", out)
	}

	runConfigure(t, "use", "lab")
	if got := currentProfile(t); got != "lab" {
		t.Fatalf("after 'use lab': current = %q", got)
	}
	if err := runConfigureExpectError(t, "use", "ghost"); !strings.Contains(err, `"ghost" is not configured`) {
		t.Fatalf("use of an unknown profile: %s", err)
	}
}

// configure and show select the profile like every other command:
// --profile, then $XOA_PROFILE, then the current profile.
func TestConfigureHonoursProfileEnvironment(t *testing.T) {
	isolate(t)
	runConfigure(t, "--profile", "prod", "--endpoint", "https://prod.example", "--token", "p")

	t.Setenv("XOA_PROFILE", "lab")
	runConfigure(t, "--endpoint", "https://lab.example", "--token", "l")
	if got := storedProfile(t, "lab"); got.Endpoint != "https://lab.example" {
		t.Fatalf("$XOA_PROFILE should select the stored profile: %+v", got)
	}
	if got := storedProfile(t, "prod"); got.Endpoint != "https://prod.example" {
		t.Fatalf("prod must be untouched: %+v", got)
	}
	if out := runConfigure(t, "show"); !strings.Contains(out, "https://lab.example") {
		t.Fatalf("show should follow $XOA_PROFILE, got:\n%s", out)
	}

	// Without --profile or $XOA_PROFILE, the current profile is updated.
	t.Setenv("XOA_PROFILE", "")
	runConfigure(t, "--token", "rotated")
	if got := storedProfile(t, "prod"); got.Token != "rotated" {
		t.Fatalf("the current profile should be updated: %+v", got)
	}
}
