package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestConfirmYesSkipsThePrompt(t *testing.T) {
	cmd := &cobra.Command{}
	var errOut strings.Builder
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader(""))
	ok, err := Confirm(cmd, "Delete?", true)
	if err != nil || !ok {
		t.Fatalf("Confirm with yes = %v, %v; want true, nil", ok, err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("no prompt expected with yes, got %q", errOut.String())
	}
}

func TestConfirmRefusesWithoutTerminal(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("y\n"))
	ok, err := Confirm(cmd, "Delete?", false)
	if ok || err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("Confirm without a terminal = %v, %v; want false and a --yes hint", ok, err)
	}
}

func TestParseID(t *testing.T) {
	const id = "550e8400-e29b-41d4-a716-446655440001"
	u, err := ParseID("VM", id)
	if err != nil || u.String() != id {
		t.Fatalf("ParseID(%q) = %v, %v", id, u, err)
	}
	if _, err := ParseID("VM", "nope"); err == nil || err.Error() != `invalid VM id "nope" (expected a UUID)` {
		t.Fatalf("ParseID(invalid) error = %v", err)
	}
}
