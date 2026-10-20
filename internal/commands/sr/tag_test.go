package sr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/testenv"
	"github.com/littlejo/xo-gocli/internal/testutil"
)

// tagSRID is the id of fixtureSR, served on the existence lookup.
const tagSRID = "11111111-1111-4111-8111-111111111111"

// tagServer is a fake XO server for the SR tag endpoints. It records the
// PUT/DELETE tag request so tests can assert on method and path, serves the
// SR fixture on the existence lookup, and 404s for any other SR id.
type tagServer struct {
	*httptest.Server
	method string
	path   string
}

func newTagServer(t *testing.T) *tagServer {
	t.Helper()
	s := &tagServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !testutil.Authorized(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/srs/"):
			if r.URL.Path != "/rest/v0/srs/"+tagSRID {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"not found"}`)
				return
			}
			_, _ = fmt.Fprint(w, fixtureSR)
		case (r.Method == http.MethodPut || r.Method == http.MethodDelete) && strings.Contains(r.URL.Path, "/tags/"):
			s.method = r.Method
			s.path = r.URL.Path
			_, _ = fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func newTagTestRoot() *cobra.Command {
	return testutil.Root(newTagCommand())
}

func runTagCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return testutil.ExecuteCombined(newTagTestRoot(), args...)
}

func TestSRTagAdd(t *testing.T) {
	server := newTagServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runTagCmd(t, "tag", "add", tagSRID, "production")
	if err != nil {
		t.Fatalf("sr tag add: %v", err)
	}
	if !strings.Contains(out, "production") || !strings.Contains(out, "Local storage") {
		t.Fatalf("tag add output should mention the tag and the SR:\n%s", out)
	}
	if server.method != http.MethodPut {
		t.Fatalf("expected a PUT request, got %s", server.method)
	}
	if server.path != "/rest/v0/srs/"+tagSRID+"/tags/production" {
		t.Fatalf("unexpected tag path: %s", server.path)
	}
}

func TestSRTagRemove(t *testing.T) {
	server := newTagServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runTagCmd(t, "tag", "remove", tagSRID, "production")
	if err != nil {
		t.Fatalf("sr tag remove: %v", err)
	}
	if !strings.Contains(out, "production") || !strings.Contains(out, "Local storage") {
		t.Fatalf("tag remove output should mention the tag and the SR:\n%s", out)
	}
	if server.method != http.MethodDelete {
		t.Fatalf("expected a DELETE request, got %s", server.method)
	}
	if server.path != "/rest/v0/srs/"+tagSRID+"/tags/production" {
		t.Fatalf("unexpected tag path: %s", server.path)
	}
}

func TestSRTagNotFound(t *testing.T) {
	server := newTagServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	if _, err := runTagCmd(t, "tag", "add", missing, "production"); err == nil {
		t.Fatal("expected a not-found error when the SR does not exist")
	}
	if server.method != "" {
		t.Fatalf("tag request must not be sent for a missing SR, got %s", server.method)
	}
}

func TestSRTagInvalidID(t *testing.T) {
	server := newTagServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	if _, err := runTagCmd(t, "tag", "add", "not-a-uuid", "production"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
	if server.method != "" {
		t.Fatalf("tag request must not be sent for an invalid id, got %s", server.method)
	}
}
