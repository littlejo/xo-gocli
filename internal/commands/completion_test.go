package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/testenv"
)

// complete runs cobra's hidden __complete command on the real root, as the
// shell completion scripts do, and returns its stdout.
func complete(t *testing.T, args ...string) string {
	t.Helper()
	root := NewRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&strings.Builder{})
	root.SetArgs(append([]string{"__complete"}, args...))
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("__complete %v: %v", args, err)
	}
	return out.String()
}

func TestCompleteVMIDs(t *testing.T) {
	var lists int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v0/vms" {
			http.NotFound(w, r)
			return
		}
		lists++
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[
			{"id":"550e8400-e29b-41d4-a716-446655440001","name_label":"web-01"},
			{"id":"660e8400-e29b-41d4-a716-446655440002","name_label":"db-01"},
			{"id":"770e8400-e29b-41d4-a716-446655440003","name_label":"twin"},
			{"id":"880e8400-e29b-41d4-a716-446655440004","name_label":"twin"}]`)
	}))
	defer srv.Close()
	testenv.Isolate(t, srv.URL)

	out := complete(t, "vm", "start", "55")
	if !strings.Contains(out, "550e8400-e29b-41d4-a716-446655440001\tweb-01") {
		t.Fatalf("expected the web-01 id with its name:\n%s", out)
	}
	if strings.Contains(out, "660e8400") {
		t.Fatalf("ids not matching the typed prefix must be left out:\n%s", out)
	}
	if lists != 1 {
		t.Fatalf("completion made %d list requests, want 1", lists)
	}
	lists = 0

	// get takes a name too: names complete as well, the id as description;
	// a name two VMs share is left out, since get would reject it.
	out = complete(t, "vm", "get", "we")
	if !strings.Contains(out, "web-01\t550e8400-e29b-41d4-a716-446655440001") {
		t.Fatalf("vm get should complete names:\n%s", out)
	}
	if out := complete(t, "vm", "get", "tw"); strings.Contains(out, "twin") {
		t.Fatalf("a shared name must not be completed:\n%s", out)
	}
	// start takes an id only: no names.
	if out := complete(t, "vm", "start", "we"); strings.Contains(out, "web-01") {
		t.Fatalf("vm start must not complete names:\n%s", out)
	}

	// Nested commands too, and only the first argument.
	if out := complete(t, "vm", "tag", "add", ""); !strings.Contains(out, "db-01") {
		t.Fatalf("tag add should complete VM ids:\n%s", out)
	}
	if out := complete(t, "vm", "tag", "add", "550e8400-e29b-41d4-a716-446655440001", ""); strings.Contains(out, "web-01") {
		t.Fatalf("the tag argument must not complete ids:\n%s", out)
	}
}

func TestCompleteWithoutServerCompletesNothing(t *testing.T) {
	testenv.Isolate(t, "http://127.0.0.1:1")
	out := complete(t, "host", "get", "")
	// Only cobra's directive line (":4", no file completion), no candidate.
	if strings.TrimSpace(out) != ":4" {
		t.Fatalf("an unreachable server must complete nothing:\n%s", out)
	}
}
