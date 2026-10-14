package token

import (
	"testing"

	"github.com/littlejo/xo-gocli/internal/testenv"
)

// TestMain runs the package's tests without the developer's XOA_* variables
// or config file (see internal/testenv).
func TestMain(m *testing.M) { testenv.Main(m) }
