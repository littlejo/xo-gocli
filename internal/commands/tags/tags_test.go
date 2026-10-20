package tags

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/testenv"
)

const testID = "11111111-1111-4111-8111-111111111111"

// recorder stubs the SDK calls of a Spec and records the tag operations.
type recorder struct {
	name    string
	nameErr error
	opErr   error
	calls   []string
}

func (r *recorder) spec() Spec {
	return Spec{
		Group:     "widget",
		Kind:      "widget",
		Noun:      "a widget",
		ExampleID: testID,
		NameOf: func(context.Context, library.Library, uuid.UUID) (string, error) {
			return r.name, r.nameErr
		},
		Add: func(_ context.Context, _ library.Library, id uuid.UUID, tag string) error {
			r.calls = append(r.calls, "add "+id.String()+" "+tag)
			return r.opErr
		},
		Remove: func(_ context.Context, _ library.Library, id uuid.UUID, tag string) error {
			r.calls = append(r.calls, "remove "+id.String()+" "+tag)
			return r.opErr
		},
	}
}

func execute(t *testing.T, spec Spec, args ...string) (stdout string, err error) {
	t.Helper()
	// The stubs never reach the API; the server only backs the client build.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{}`)
	}))
	t.Cleanup(server.Close)
	testenv.Isolate(t, server.URL)

	root := &cobra.Command{Use: "xo", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.AddCommand(NewCommand(spec))
	var out, errOut strings.Builder
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err = root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestAddAndRemove(t *testing.T) {
	for _, tc := range []struct{ verb, want string }{
		{"add", `Tag "prod" added on widget "w-01"`},
		{"remove", `Tag "prod" removed on widget "w-01"`},
	} {
		r := &recorder{name: "w-01"}
		out, err := execute(t, r.spec(), "tag", tc.verb, testID, "prod")
		if err != nil {
			t.Fatalf("tag %s: %v", tc.verb, err)
		}
		if strings.TrimSpace(out) != tc.want {
			t.Fatalf("tag %s output = %q, want %q", tc.verb, out, tc.want)
		}
		if len(r.calls) != 1 || r.calls[0] != tc.verb+" "+testID+" prod" {
			t.Fatalf("tag %s calls = %v", tc.verb, r.calls)
		}
	}
}

func TestEmptyNameFallsBackToID(t *testing.T) {
	r := &recorder{}
	out, err := execute(t, r.spec(), "tag", "add", testID, "prod")
	if err != nil {
		t.Fatalf("tag add: %v", err)
	}
	if !strings.Contains(out, `widget "`+testID+`"`) {
		t.Fatalf("an unnamed widget should be shown by id:\n%s", out)
	}
}

func TestInvalidIDSkipsTheCall(t *testing.T) {
	r := &recorder{name: "w-01"}
	_, err := execute(t, r.spec(), "tag", "add", "not-a-uuid", "prod")
	if err == nil || !strings.Contains(err.Error(), `invalid widget id "not-a-uuid"`) {
		t.Fatalf("expected an invalid id error, got %v", err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("no tag operation expected, got %v", r.calls)
	}
}

func TestMissingResourceSkipsTheCall(t *testing.T) {
	r := &recorder{nameErr: errors.New("boom")}
	_, err := execute(t, r.spec(), "tag", "add", testID, "prod")
	if err == nil || !strings.Contains(err.Error(), "cannot resolve widget") {
		t.Fatalf("expected a resolve error, got %v", err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("no tag operation expected, got %v", r.calls)
	}
}

func TestOperationError(t *testing.T) {
	r := &recorder{name: "w-01", opErr: errors.New("forbidden")}
	_, err := execute(t, r.spec(), "tag", "remove", testID, "prod")
	if err == nil || !strings.Contains(err.Error(), `cannot remove tag "prod" on widget "w-01"`) {
		t.Fatalf("expected the operation error, got %v", err)
	}
}

func TestHelp(t *testing.T) {
	cmd := NewCommand((&recorder{}).spec())
	if cmd.Short != "Manage the tags of a widget" {
		t.Fatalf("group Short = %q", cmd.Short)
	}
	add, _, err := cmd.Find([]string{"add"})
	if err != nil {
		t.Fatal(err)
	}
	if add.Short != "Add a tag to a widget" || !strings.Contains(add.Long, "'xo widget list'") {
		t.Fatalf("add help: %q / %q", add.Short, add.Long)
	}
	remove, _, err := cmd.Find([]string{"remove"})
	if err != nil {
		t.Fatal(err)
	}
	if remove.Short != "Remove a tag from a widget" || !strings.Contains(remove.Example, "xo widget tag remove "+testID) {
		t.Fatalf("remove help: %q / %q", remove.Short, remove.Example)
	}
}
