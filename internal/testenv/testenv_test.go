package testenv

import (
	"os"
	"testing"
)

func TestClearKeepsIntegrationSettings(t *testing.T) {
	t.Setenv("XOA_DEFAULT_OUTPUT", "json")
	t.Setenv("XOA_TEST_URL", "http://xo.test")

	Clear()

	if v, ok := os.LookupEnv("XOA_DEFAULT_OUTPUT"); ok {
		t.Errorf("XOA_DEFAULT_OUTPUT should be unset, got %q", v)
	}
	if got := os.Getenv("XOA_TEST_URL"); got != "http://xo.test" {
		t.Errorf("XOA_TEST_URL = %q, want it kept for the integration tests", got)
	}
}
