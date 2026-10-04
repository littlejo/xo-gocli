package template

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// listTestTemplates all belong to the same pool.
const listTestTemplates = `[
	{"id":"d31e47fd-a70e-d849-883e-c17193472710-6959dfe8-534c-4c58-8a8c-3c3792293543","name_label":"Oracle Linux 8","type":"VM-template","isDefaultTemplate":true,"power_state":"Halted","memory":{"size":4294967296},"CPUs":{"number":2,"max":2},"$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009"},
	{"id":"d31e47fd-a70e-d849-883e-c17193472710-7aa32be8-a06c-4ade-8a1d-49e51e03e9d2","name_label":"AlmaLinux 8","type":"VM-template","isDefaultTemplate":false,"power_state":"Halted","memory":{"size":2147483648},"CPUs":{"number":1,"max":1},"$pool":"aaaaaaaa-bbbb-cccc-dddd-000000000009"}
]`

// listDetailServer serves the vm-templates collection (a raw REST resource)
// plus the pools collection for the pool names, and records every request
// path so tests can assert the batch cost.
type listDetailServer struct {
	*httptest.Server
	paths []string
}

func newListDetailServer(t *testing.T) *listDetailServer {
	t.Helper()
	s := &listDetailServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.paths = append(s.paths, r.URL.Path)
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/v0/vm-templates":
			_, _ = fmt.Fprint(w, listTestTemplates)
		case "/rest/v0/pools":
			_, _ = fmt.Fprint(w, `[
				{"id":"aaaaaaaa-bbbb-cccc-dddd-000000000009","name_label":"prod-pool"}
			]`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *listDetailServer) count(prefix string) int {
	p := "/rest/v0/" + prefix
	n := 0
	for _, x := range s.paths {
		if strings.HasPrefix(x, p) {
			n++
		}
	}
	return n
}

// The table shows the pool each template belongs to by name.
func TestTemplateListResolved(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}
	if !strings.Contains(out, "prod-pool") {
		t.Fatalf("list output missing the pool name:\n%s", out)
	}
	if strings.Contains(out, "aaaaaaaa-bbbb-cccc-dddd-000000000009") {
		t.Fatalf("list output still shows the raw pool id:\n%s", out)
	}
}

// Every pool is resolved with a single batch — never one lookup per template.
func TestTemplateListCostConstant(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	if _, err := runList(t, "list"); err != nil {
		t.Fatalf("template list: %v", err)
	}
	if n := srv.count("pools"); n != 1 {
		t.Fatalf("expected exactly 1 pool batch, got %d", n)
	}
}

// --output json keeps the raw pool reference and makes no batch request.
func TestTemplateListJSONNoBatches(t *testing.T) {
	srv := newListDetailServer(t)
	isolatePointers(t, srv.URL)

	out, err := runList(t, "list", "--output", "json")
	if err != nil {
		t.Fatalf("template list --output json: %v", err)
	}
	if !strings.Contains(out, `"aaaaaaaa-bbbb-cccc-dddd-000000000009"`) {
		t.Fatalf("json output must keep the raw pool reference:\n%s", out)
	}
	if n := srv.count("pools"); n != 0 {
		t.Fatalf("expected no batch request for json output, got %d", n)
	}
}
