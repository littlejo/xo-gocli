// Package testenv isolates the tests from the developer's own xo setup.
//
// Every XOA_* variable changes the CLI's behavior (XOA_DEFAULT_OUTPUT=json
// alone turns every table assertion red), and the default config file holds
// the developer's real profiles. Each test package therefore starts from
// Main, which clears the whole XOA_ namespace and points the config file at
// an empty temporary directory, so 'go test ./...' gives the same result on
// any machine.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// envPrefix is the namespace of every variable the CLI reads.
const envPrefix = "XOA_"

// keepPrefix marks the variables that configure the opt-in integration
// tests (XOA_TEST_URL, XOA_TEST_TOKEN): the CLI never reads them, and
// clearing them would silently skip the integration suite.
const keepPrefix = "XOA_TEST_"

// envConfigFile overrides the config file path (see config.Path).
const envConfigFile = "XOA_CONFIG_FILE"

// Main runs the package's tests in a clean environment (see Clear). Use it
// as the package's TestMain:
//
//	func TestMain(m *testing.M) { testenv.Main(m) }
func Main(m *testing.M) {
	Clear()
	dir, err := os.MkdirTemp("", "xo-test-config-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv:", err)
		os.Exit(1)
	}
	// A path that does not exist yet: the CLI sees "no profile configured"
	// unless a test writes one, never the developer's ~/.config/xo/config.
	if err := os.Setenv(envConfigFile, filepath.Join(dir, "config")); err != nil {
		fmt.Fprintln(os.Stderr, "testenv:", err)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// Clear unsets every XOA_* variable of the current process, except the
// XOA_TEST_* integration settings.
func Clear() {
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, envPrefix) && !strings.HasPrefix(key, keepPrefix) {
			_ = os.Unsetenv(key)
		}
	}
}

// Isolate points the CLI at a fake Xen Orchestra server for one test: a
// throwaway config file, url as the endpoint and a test token. Main has
// already cleared every other XOA_* variable; the values set here are
// restored when the test ends.
func Isolate(t *testing.T, url string) {
	t.Helper()
	t.Setenv(envConfigFile, filepath.Join(t.TempDir(), "config"))
	t.Setenv("XOA_ENDPOINT", url)
	t.Setenv("XOA_TOKEN", "test-token")
}
