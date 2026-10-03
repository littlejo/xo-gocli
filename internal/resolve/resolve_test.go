package resolve

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/uuid"

	sdkconfig "github.com/vatesfr/xenorchestra-go-sdk/pkg/config"
	"github.com/vatesfr/xenorchestra-go-sdk/v2"
	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"
)

// requestLog records every request path seen by the fake server. The handler
// appends from the server's goroutine while the test reads the count, so it is
// guarded (network I/O is not a synchronization edge for the race detector).
type requestLog struct {
	mu    sync.Mutex
	paths []string
}

func (l *requestLog) add(p string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paths = append(l.paths, p)
}

func (l *requestLog) count(prefix string) int {
	p := "/rest/v0/" + prefix
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, x := range l.paths {
		if strings.HasPrefix(x, p) {
			n++
		}
	}
	return n
}

func (l *requestLog) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.paths)
}

// fakeXO serves the minimal REST surface the resolver uses. Routes are keyed
// by the path prefix after /rest/v0 (matched by prefix). The auth cookie is
// enforced; every request is recorded in the returned log so tests can assert
// how many times each endpoint was hit (the anti-N+1 contract).
func fakeXO(t *testing.T, routes map[string]func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *requestLog) {
	t.Helper()
	log := &requestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r.URL.Path)
		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}
		for prefix, h := range routes {
			if strings.HasPrefix(r.URL.Path, "/rest/v0/"+prefix) {
				h(w, r)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"object not found"}`)
	}))
	t.Cleanup(srv.Close)
	return srv, log
}

// newClient builds a resolver backed by a real SDK v2 client pointed at the
// fake server.
func newClient(t *testing.T, url string) *Client {
	t.Helper()
	cfg, err := sdkconfig.NewWithValues(&sdkconfig.Config{
		Url:                 url,
		Token:               "test-token",
		ClientTimeout:       10 * time.Second,
		LogOutputPaths:      []string{"/dev/null"},
		LogErrorOutputPaths: []string{"/dev/null"},
	})
	if err != nil {
		t.Fatalf("sdk config: %v", err)
	}
	lib, err := v2.New(cfg)
	if err != nil {
		t.Fatalf("sdk library: %v", err)
	}
	httpClient, err := client.New(cfg)
	if err != nil {
		t.Fatalf("sdk rest client: %v", err)
	}
	return New(lib, httpClient)
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	u, err := uuid.FromString(s)
	if err != nil {
		t.Fatalf("uuid %q: %v", s, err)
	}
	return u
}

// json writes a JSON body with the given status (0 → 200).
func json(w http.ResponseWriter, status int, body string) {
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprint(w, body)
}

func TestHostNameCaches(t *testing.T) {
	id := mustUUID(t, "aaaaaaaa-0000-0000-0000-000000000001")
	srv, log := fakeXO(t, map[string]func(http.ResponseWriter, *http.Request){
		"hosts/": func(w http.ResponseWriter, _ *http.Request) {
			json(w, 0, `{"id":"`+id.String()+`","name_label":"host-01"}`)
		},
	})
	c := newClient(t, srv.URL)

	name, err := c.HostName(context.Background(), id)
	if err != nil {
		t.Fatalf("HostName: %v", err)
	}
	if name != "host-01" {
		t.Fatalf("expected host-01, got %q", name)
	}
	// A second call must be served from the cache, not the API.
	if _, err := c.HostName(context.Background(), id); err != nil {
		t.Fatalf("HostName (cached): %v", err)
	}
	if n := log.count("hosts/"); n != 1 {
		t.Fatalf("expected 1 host fetch, got %d", n)
	}
}

func TestPoolNameCaches(t *testing.T) {
	id := mustUUID(t, "bbbbbbbb-0000-0000-0000-000000000001")
	srv, log := fakeXO(t, map[string]func(http.ResponseWriter, *http.Request){
		"pools/": func(w http.ResponseWriter, _ *http.Request) {
			json(w, 0, `{"id":"`+id.String()+`","name_label":"prod-pool"}`)
		},
	})
	c := newClient(t, srv.URL)

	name, err := c.PoolName(context.Background(), id)
	if err != nil || name != "prod-pool" {
		t.Fatalf("expected prod-pool, got %q (%v)", name, err)
	}
	if _, err := c.PoolName(context.Background(), id); err != nil {
		t.Fatalf("PoolName (cached): %v", err)
	}
	if n := log.count("pools/"); n != 1 {
		t.Fatalf("expected 1 pool fetch, got %d", n)
	}
}

func TestSRNameCaches(t *testing.T) {
	id := mustUUID(t, "cccccccc-0000-0000-0000-000000000001")
	srv, log := fakeXO(t, map[string]func(http.ResponseWriter, *http.Request){
		"srs/": func(w http.ResponseWriter, _ *http.Request) {
			json(w, 0, `{"id":"`+id.String()+`","name_label":"Local Storage"}`)
		},
	})
	c := newClient(t, srv.URL)

	name, err := c.SRName(context.Background(), id)
	if err != nil || name != "Local Storage" {
		t.Fatalf("expected Local Storage, got %q (%v)", name, err)
	}
	if n := log.count("srs/"); n != 1 {
		t.Fatalf("expected 1 SR fetch, got %d", n)
	}
}

func TestVMNameCaches(t *testing.T) {
	id := mustUUID(t, "dddddddd-0000-0000-0000-000000000001")
	srv, log := fakeXO(t, map[string]func(http.ResponseWriter, *http.Request){
		"vms/": func(w http.ResponseWriter, _ *http.Request) {
			json(w, 0, `{"id":"`+id.String()+`","name_label":"web-01"}`)
		},
	})
	c := newClient(t, srv.URL)

	name, err := c.VMName(context.Background(), id)
	if err != nil || name != "web-01" {
		t.Fatalf("expected web-01, got %q (%v)", name, err)
	}
	if _, err := c.VMName(context.Background(), id); err != nil {
		t.Fatalf("VMName (cached): %v", err)
	}
	if n := log.count("vms/"); n != 1 {
		t.Fatalf("expected 1 VM fetch, got %d", n)
	}
}

// ResolveContainer must try the host first and only then the pool, with a
// constant cost (no more than one request per candidate kind).
func TestResolveContainerHost(t *testing.T) {
	srv, log := fakeXO(t, map[string]func(http.ResponseWriter, *http.Request){
		"hosts/": func(w http.ResponseWriter, _ *http.Request) {
			json(w, 0, `{"name_label":"host-02"}`)
		},
		"pools/": func(w http.ResponseWriter, _ *http.Request) {
			t.Fatal("pools/ must not be hit when the container is a host")
		},
	})
	c := newClient(t, srv.URL)

	name, err := c.ResolveContainer(context.Background(), mustUUID(t, "aaaaaaaa-0000-0000-0000-000000000001"))
	if err != nil || name != "host-02" {
		t.Fatalf("expected host-02, got %q (%v)", name, err)
	}
	if n := log.count("pools/"); n != 0 {
		t.Fatalf("expected 0 pool fetch, got %d", n)
	}
}

func TestResolveContainerPool(t *testing.T) {
	srv, log := fakeXO(t, map[string]func(http.ResponseWriter, *http.Request){
		"hosts/": func(w http.ResponseWriter, _ *http.Request) {
			json(w, http.StatusNotFound, `{"message":"object not found"}`)
		},
		"pools/": func(w http.ResponseWriter, _ *http.Request) {
			json(w, 0, `{"name_label":"prod-pool"}`)
		},
	})
	c := newClient(t, srv.URL)

	name, err := c.ResolveContainer(context.Background(), mustUUID(t, "bbbbbbbb-0000-0000-0000-000000000001"))
	if err != nil || name != "prod-pool" {
		t.Fatalf("expected prod-pool, got %q (%v)", name, err)
	}
	if n := log.count("hosts/") + log.count("pools/"); n != 2 {
		t.Fatalf("expected 2 requests (host then pool), got %d", n)
	}
}

func TestResolveContainerUnknown(t *testing.T) {
	srv, _ := fakeXO(t, map[string]func(http.ResponseWriter, *http.Request){
		"hosts/": func(w http.ResponseWriter, _ *http.Request) { json(w, http.StatusNotFound, `{}`) },
		"pools/": func(w http.ResponseWriter, _ *http.Request) { json(w, http.StatusNotFound, `{}`) },
	})
	c := newClient(t, srv.URL)

	id := mustUUID(t, "eeeeeeee-0000-0000-0000-000000000001")
	name, err := c.ResolveContainer(context.Background(), id)
	if err == nil {
		t.Fatal("expected an error for an unknown container")
	}
	if name != id.String() {
		t.Fatalf("expected raw id fallback, got %q", name)
	}
}

func TestTemplateResolves(t *testing.T) {
	id := "d31e47fd-a70e-d849-883e-c17193472710-6959dfe8-534c-4c58-8a8c-3c3792293543"
	srv, log := fakeXO(t, map[string]func(http.ResponseWriter, *http.Request){
		"vm-templates/": func(w http.ResponseWriter, _ *http.Request) {
			json(w, 0, `{"id":"`+id+`","name_label":"Oracle Linux 8"}`)
		},
	})
	c := newClient(t, srv.URL)

	name, err := c.Template(context.Background(), id)
	if err != nil || name != "Oracle Linux 8" {
		t.Fatalf("expected Oracle Linux 8, got %q (%v)", name, err)
	}
	// Second call is cached.
	if _, err := c.Template(context.Background(), id); err != nil {
		t.Fatalf("Template (cached): %v", err)
	}
	if n := log.count("vm-templates/"); n != 1 {
		t.Fatalf("expected 1 template fetch, got %d", n)
	}
}

func TestTemplateMissing(t *testing.T) {
	id := "d31e47fd-a70e-d849-883e-c17193472710-0000000000000000000000000000"
	srv, _ := fakeXO(t, map[string]func(http.ResponseWriter, *http.Request){
		"vm-templates/": func(w http.ResponseWriter, _ *http.Request) {
			json(w, http.StatusNotFound, `{"message":"object not found"}`)
		},
	})
	c := newClient(t, srv.URL)

	name, err := c.Template(context.Background(), id)
	if err == nil {
		t.Fatal("expected an error for a missing template")
	}
	if name != id {
		t.Fatalf("expected raw id fallback, got %q", name)
	}
}

// Nil and empty references resolve to themselves without hitting the API.
func TestNilReferenceNoFetch(t *testing.T) {
	srv, log := fakeXO(t, map[string]func(http.ResponseWriter, *http.Request){})
	c := newClient(t, srv.URL)

	if name, err := c.HostName(context.Background(), uuid.Nil); err != nil || name != uuid.Nil.String() {
		t.Fatalf("expected nil host fallback, got %q (%v)", name, err)
	}
	if name, err := c.PoolName(context.Background(), uuid.Nil); err != nil || name != uuid.Nil.String() {
		t.Fatalf("expected nil pool fallback, got %q (%v)", name, err)
	}
	if name, err := c.Template(context.Background(), ""); err != nil || name != "" {
		t.Fatalf("expected empty template fallback, got %q (%v)", name, err)
	}
	if n := log.len(); n != 0 {
		t.Fatalf("expected no API request, got %d", n)
	}
}
