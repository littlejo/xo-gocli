package vm

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// These tests prove the error-translation layer (internal/cli) end to end:
// the concise message must reach the user through a real command, with no
// Go/SDK internals leaking, while --debug can still reveal the raw error via
// cli.Detail. They complement the unit tests in internal/cli.

// fakeSlowXO answers only after sleeping, so a client timeout shorter than
// the sleep fails.
func fakeSlowXO(t *testing.T, sleep time.Duration) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(sleep)
		if r.URL.Path != "/rest/v0/vms" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
}

func TestVMListTimeoutIsTranslated(t *testing.T) {
	server := fakeSlowXO(t, 500*time.Millisecond)
	defer server.Close()
	isolatePointers(t, server.URL)
	t.Setenv(cli.EnvTimeout, "50ms")

	_, err := runList(t, "list")
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if !strings.Contains(err.Error(), "the request timed out after 50ms") {
		t.Fatalf("timeout not translated: %v", err)
	}
	for _, internal := range []string{"context deadline", "Client.Timeout", "failed to do request"} {
		if strings.Contains(err.Error(), internal) {
			t.Fatalf("Go internals leaked: %q in %v", internal, err)
		}
	}
	if d := cli.Detail(err); !strings.Contains(d, "context deadline exceeded") {
		t.Fatalf("the raw error must be kept as the debug detail, got: %q", d)
	}
}

func TestVMListUnreachableIsTranslated(t *testing.T) {
	// Port 1 on the loopback is not listening: the dial is refused.
	isolatePointers(t, "http://127.0.0.1:1")
	t.Setenv(cli.EnvTimeout, "")

	_, err := runList(t, "list")
	if err == nil {
		t.Fatal("expected a connection error")
	}
	if !strings.Contains(err.Error(), "cannot reach 127.0.0.1:1: connection refused") {
		t.Fatalf("dial error not translated: %v", err)
	}
	if strings.Contains(err.Error(), "dial tcp") {
		t.Fatalf("Go internals leaked: %v", err)
	}
}

func TestVMListMalformedResponseIsTranslated(t *testing.T) {
	// power_state must be a string; a number fails to unmarshal into the
	// SDK payload.
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"550e8400-e29b-41d4-a716-446655440001","name_label":"web-01","power_state":123,"memory":{"size":1024},"CPUs":{"number":1},"boot":{},"type":"vm"}]`))
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runList(t, "list")
	if err == nil {
		t.Fatal("expected an unmarshal error")
	}
	if !strings.Contains(err.Error(), "the Xen Orchestra API returned a malformed response") {
		t.Fatalf("unmarshal error not translated: %v", err)
	}
	if strings.Contains(err.Error(), "cannot unmarshal") || strings.Contains(err.Error(), "failed to unmarshal") {
		t.Fatalf("Go internals leaked: %v", err)
	}
	if d := cli.Detail(err); !strings.Contains(d, "cannot unmarshal") {
		t.Fatalf("the raw error must be kept as the debug detail, got: %q", d)
	}
}

func TestVMListRejectedTokenIsTranslated(t *testing.T) {
	server := fakeXO(t, nil) // verifies the authenticationToken cookie
	defer server.Close()
	isolatePointers(t, server.URL)
	t.Setenv("XOA_TOKEN", "a-wrong-token")

	_, err := runList(t, "list")
	if err == nil {
		t.Fatal("expected a 401 error")
	}
	if !strings.Contains(err.Error(), "authentication was rejected by the server (401)") {
		t.Fatalf("401 not translated: %v", err)
	}
	if strings.Contains(err.Error(), "API error") {
		t.Fatalf("the raw API form must not remain: %v", err)
	}
	if strings.Contains(err.Error(), "a-wrong-token") {
		t.Fatalf("the rejected token must not leak into the error: %v", err)
	}
}

// A 500 whose body merely contains "404" must still be reported as a server
// error, not as a not-found lookup.
func TestVMList500With404InBodyIsNotNotFound(t *testing.T) {
	server := fakeXO(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"inner 404"}`))
	})
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := runList(t, "list")
	if err == nil {
		t.Fatal("expected a 500 error")
	}
	if strings.Contains(err.Error(), "not found") {
		t.Fatalf("a 500 mentioning 404 in its body must not be 'not found': %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("the 500 status should remain visible: %v", err)
	}
}
