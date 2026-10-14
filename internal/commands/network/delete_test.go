package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/littlejo/xo-gocli/internal/testenv"
)

// deleteNetworkID is the id of fixtureNetwork, served on the existence lookup.
const deleteNetworkID = "11111111-1111-4111-8111-111111111111"

// deleteServer is a fake XO server for the network delete endpoint. It
// answers:
//
//	GET    /rest/v0/networks/{id} -> fixtureNetwork (404 for other ids)
//	DELETE /rest/v0/networks/{id} -> 200 {}
//
// and records requests so tests can assert the delete actually went out (and
// that it did not when confirmation was refused or the id was invalid).
type deleteServer struct {
	*httptest.Server
	deleted bool
}

func newDeleteServer(t *testing.T) *deleteServer {
	t.Helper()
	s := &deleteServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/networks/"+deleteNetworkID:
			_, _ = fmt.Fprint(w, fixtureNetwork)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/v0/networks/"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/v0/networks/"+deleteNetworkID:
			s.deleted = true
			_, _ = fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func TestNetworkDelete(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runNetwork(t, "delete", deleteNetworkID, "--yes")
	if err != nil {
		t.Fatalf("network delete: %v", err)
	}
	if !strings.Contains(out, "Management") {
		t.Fatalf("delete output should mention the network name:\n%s", out)
	}
	if !server.deleted {
		t.Fatal("expected a DELETE request")
	}
}

func TestNetworkDeleteJSON(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	out, err := runNetwork(t, "delete", deleteNetworkID, "--yes", "--output", "json")
	if err != nil {
		t.Fatalf("network delete --output json: %v", err)
	}
	// Machine output must be valid JSON carrying only the requested data.
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("delete JSON output is not valid JSON: %v\n%s", err, out)
	}
	if doc["network"] != "Management" {
		t.Fatalf("unexpected delete JSON payload: %s", out)
	}
}

func TestNetworkDeleteRequiresConfirmation(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	// Without --yes and with a non-terminal stdin, the delete must be refused
	// before any request is sent.
	if _, err := runNetwork(t, "delete", deleteNetworkID); err == nil {
		t.Fatal("expected an error when no confirmation is given and stdin is not a terminal")
	}
	if server.deleted {
		t.Fatal("the network must not be deleted without confirmation")
	}
}

func TestNetworkDeleteSkipsConfirmationWithEnvYes(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)
	t.Setenv("XOA_YES", "1")

	if _, err := runNetwork(t, "delete", deleteNetworkID); err != nil {
		t.Fatalf("network delete with XOA_YES=1: %v", err)
	}
	if !server.deleted {
		t.Fatal("expected a DELETE request when XOA_YES=1")
	}
}

func TestNetworkDeleteBadID(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	if _, err := runNetwork(t, "delete", "not-a-uuid", "--yes"); err == nil {
		t.Fatal("expected an error for an invalid id")
	}
	if server.deleted {
		t.Fatal("the network must not be deleted for an invalid id")
	}
}

func TestNetworkDeleteNotFound(t *testing.T) {
	server := newDeleteServer(t)
	defer server.Close()
	testenv.Isolate(t, server.URL)

	missing := "99999999-9999-4999-8999-999999999999"
	_, err := runNetwork(t, "delete", missing, "--yes")
	if err == nil {
		t.Fatal("expected an error when the network does not exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if server.deleted {
		t.Fatal("the network must not be deleted when it does not exist")
	}
}
