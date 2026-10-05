package network

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// tagNetworkID is the id of fixtureNetwork, served on the existence lookup.
const tagNetworkID = "11111111-1111-4111-8111-111111111111"

// tagServer is a fake XO server for the network tag endpoints. It records the
// PUT/DELETE tag request so tests can assert on method and path, serves the
// network fixture on the existence lookup, and 404s for any other network id.
type tagServer struct {
	*httptest.Server
	method string
	path   string
}

func newTagServer(t *testing.T) *tagServer {
	t.Helper()
	s := &tagServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/networks/"):
			if r.URL.Path != "/rest/v0/networks/"+tagNetworkID {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"message":"not found"}`)
				return
			}
			_, _ = fmt.Fprint(w, fixtureNetwork)
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
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	root.AddCommand(newTagCommand())
	return root
}

func runTagCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newTagTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestNetworkTagAdd(t *testing.T) {
	server := newTagServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runTagCmd(t, "tag", "add", tagNetworkID, "production")
	if err != nil {
		t.Fatalf("network tag add: %v", err)
	}
	if !strings.Contains(out, "production") || !strings.Contains(out, "Management") {
		t.Fatalf("tag add output should mention the tag and the network:\n%s", out)
	}
	if server.method != http.MethodPut {
		t.Fatalf("expected a PUT request, got %s", server.method)
	}
	if server.path != "/rest/v0/networks/"+tagNetworkID+"/tags/production" {
		t.Fatalf("unexpected tag path: %s", server.path)
	}
}

func TestNetworkTagRemove(t *testing.T) {
	server := newTagServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := runTagCmd(t, "tag", "remove", tagNetworkID, "production")
	if err != nil {
		t.Fatalf("network tag remove: %v", err)
	}
	if !strings.Contains(out, "production") || !strings.Contains(out, "Management") {
		t.Fatalf("tag remove output should mention the tag and the network:\n%s", out)
	}
	if server.method != http.MethodDelete {
		t.Fatalf("expected a DELETE request, got %s", server.method)
	}
	if server.path != "/rest/v0/networks/"+tagNetworkID+"/tags/production" {
		t.Fatalf("unexpected tag path: %s", server.path)
	}
}

func TestNetworkTagNotFound(t *testing.T) {
	server := newTagServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	if _, err := runTagCmd(t, "tag", "add", missing, "production"); err == nil {
		t.Fatal("expected a not-found error when the network does not exist")
	}
	if server.method != "" {
		t.Fatalf("tag request must not be sent for a missing network, got %s", server.method)
	}
}

func TestNetworkTagInvalidID(t *testing.T) {
	server := newTagServer(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := runTagCmd(t, "tag", "add", "not-a-uuid", "production"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
	if server.method != "" {
		t.Fatalf("tag request must not be sent for an invalid id, got %s", server.method)
	}
}
