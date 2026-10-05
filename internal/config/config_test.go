package config

import (
	"os"
	"path/filepath"
	"testing"
)

func isolateConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	t.Setenv("XOA_CONFIG_FILE", path)
	// Clear any environment overrides so tests are deterministic.
	for _, key := range []string{EnvProfile, EnvEndpoint, EnvToken, EnvUsername, EnvPassword, EnvInsecure, EnvDefaultOutput} {
		t.Setenv(key, "")
	}
	return path
}

func TestLoadCarriesProfileOutput(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t", Output: "json"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Output != "json" {
		t.Fatalf("profile output not carried into the resolved config: %+v", cfg)
	}
}

func TestDefaultOutputEnvOverridesProfileOutput(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t", Output: "yaml"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	t.Setenv(EnvDefaultOutput, "json")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Output != "json" {
		t.Fatalf("$%s must override the stored profile output: %+v", EnvDefaultOutput, cfg)
	}
}

func TestDefaultOutputEnvOnly(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	t.Setenv(EnvDefaultOutput, "text")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Output != "text" {
		t.Fatalf("$%s must be used when the profile has no output: %+v", EnvDefaultOutput, cfg)
	}
}

func TestLoadWithoutProfileOutputIsEmpty(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Output != "" {
		t.Fatalf("no output configured, expected an empty value: %+v", cfg)
	}
}

func TestLoadWithoutAnyConfigurationFails(t *testing.T) {
	isolateConfig(t)

	if _, err := Load(""); err == nil {
		t.Fatal("expected an error when no endpoint is configured")
	}
}

func TestUpsertThenLoad(t *testing.T) {
	isolateConfig(t)

	profile := Profile{Name: "lab", Endpoint: "https://xo.lab.example.com", Token: "secret"}
	path, err := Upsert(profile, true)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config file was not written: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("config file is empty")
	}

	cfg, err := Load("lab")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Endpoint != profile.Endpoint || cfg.Token != profile.Token {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Name != "lab" {
		t.Fatalf("unexpected profile name: %q", cfg.Name)
	}
}

func TestUpsertReplacesExistingProfile(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://old.example.com", Token: "a"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://new.example.com", Token: "b"}, false); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	cfg, err := Load("lab")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Endpoint != "https://new.example.com" || cfg.Token != "b" {
		t.Fatalf("profile was not replaced: %+v", cfg)
	}
}

func TestLoadUnknownProfileFails(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://xo.lab.example.com", Token: "a"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if _, err := Load("missing"); err == nil {
		t.Fatal("expected an error for an unknown profile")
	}
}

func TestEnvironmentOverridesFile(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://file.example.com", Token: "file-token"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	t.Setenv(EnvEndpoint, "https://env.example.com")
	t.Setenv(EnvToken, "env-token")

	cfg, err := Load("lab")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Endpoint != "https://env.example.com" || cfg.Token != "env-token" {
		t.Fatalf("environment variables must win over the file: %+v", cfg)
	}
}

func TestProfileFromEnvironment(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "prod", Endpoint: "https://prod.example.com", Token: "t"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	t.Setenv(EnvProfile, "prod")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Name != "prod" || cfg.Endpoint != "https://prod.example.com" {
		t.Fatalf("XOA_PROFILE was not honored: %+v", cfg)
	}
}

// The file's current profile is the fallback used when neither --profile nor
// $XOA_PROFILE selects one (AWS CLI style), after the default profile is not a
// valid fallback when a current marker exists.

func TestCurrentProfileFromFile(t *testing.T) {
	isolateConfig(t)

	// "default" is never configured here; "lab" is the current profile.
	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Name != "lab" || cfg.Endpoint != "https://lab.example.com" {
		t.Fatalf("current profile from file was not used: %+v", cfg)
	}
}

func TestExplicitProfileWinsOverCurrent(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if _, err := Upsert(Profile{Name: "other", Endpoint: "https://other.example.com", Token: "t"}, false); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	cfg, err := Load("other")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Name != "other" || cfg.Endpoint != "https://other.example.com" {
		t.Fatalf("explicit profile must win over the current marker: %+v", cfg)
	}
}

func TestEnvProfileWinsOverCurrent(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if _, err := Upsert(Profile{Name: "ci", Endpoint: "https://ci.example.com", Token: "t"}, false); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	t.Setenv(EnvProfile, "ci")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Name != "ci" {
		t.Fatalf("$XOA_PROFILE must win over the current marker: %+v", cfg)
	}
}

func TestLoadRequiresCredentials(t *testing.T) {
	isolateConfig(t)

	t.Setenv(EnvEndpoint, "https://xo.example.com")
	if _, err := Load(""); err == nil {
		t.Fatal("expected an error when no credentials are configured")
	}
}

func TestListReturnsProfilesAndCurrent(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t"}, true); err != nil {
		t.Fatalf("Upsert lab: %v", err)
	}
	if _, err := Upsert(Profile{Name: "ci", Endpoint: "https://ci.example.com", Token: "t"}, false); err != nil {
		t.Fatalf("Upsert ci: %v", err)
	}

	current, profiles, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if current != "lab" {
		t.Fatalf("current = %q, want lab", current)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestListEmptyIsNotAnError(t *testing.T) {
	isolateConfig(t)

	current, profiles, err := List()
	if err != nil {
		t.Fatalf("List on empty file: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected no profiles, got %d", len(profiles))
	}
	if current != DefaultProfile {
		t.Fatalf("current = %q, want %q", current, DefaultProfile)
	}
}

func TestGetProfileReturnsStoredProfile(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "s3cret", Insecure: true}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	p, err := GetProfile("lab")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if p.Endpoint != "https://lab.example.com" || p.Token != "s3cret" || !p.Insecure {
		t.Fatalf("unexpected profile: %+v", p)
	}
}

func TestGetProfileUnknownFails(t *testing.T) {
	isolateConfig(t)

	if _, err := GetProfile("nope"); err == nil {
		t.Fatal("expected an error for an unknown profile")
	}
}

func TestRemoveDeletesProfile(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t"}, true); err != nil {
		t.Fatalf("Upsert lab: %v", err)
	}
	if _, err := Upsert(Profile{Name: "ci", Endpoint: "https://ci.example.com", Token: "t"}, false); err != nil {
		t.Fatalf("Upsert ci: %v", err)
	}

	if _, err := Remove("lab"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := GetProfile("lab"); err == nil {
		t.Fatal("lab should have been removed")
	}
	if _, err := GetProfile("ci"); err != nil {
		t.Fatalf("ci should still exist: %v", err)
	}
}

// Removing the current profile moves the current marker to the first
// remaining profile.

func TestRemoveCurrentFallsBackToFirstRemaining(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t"}, true); err != nil {
		t.Fatalf("Upsert lab: %v", err)
	}
	if _, err := Upsert(Profile{Name: "ci", Endpoint: "https://ci.example.com", Token: "t"}, false); err != nil {
		t.Fatalf("Upsert ci: %v", err)
	}

	current, err := Remove("lab")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if current != "ci" {
		t.Fatalf("after removing current, current = %q, want ci", current)
	}
}

// Removing the last profile resets current to the default profile.

func TestRemoveLastResetsCurrent(t *testing.T) {
	isolateConfig(t)

	if _, err := Upsert(Profile{Name: "lab", Endpoint: "https://lab.example.com", Token: "t"}, true); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	current, err := Remove("lab")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if current != DefaultProfile {
		t.Fatalf("after removing the last profile, current = %q, want %q", current, DefaultProfile)
	}
}

func TestRemoveUnknownFails(t *testing.T) {
	isolateConfig(t)

	if _, err := Remove("nope"); err == nil {
		t.Fatal("expected an error when removing an unknown profile")
	}
}
