package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
)

const (
	fixtureVMS = `[
		{"id": "aaaa-1111", "name_label": "web-01", "power_state": "Running"},
		{"id": "bbbb-2222", "name_label": "db-01", "power_state": "Halted"}
	]`
	fixtureVM = `{"id": "aaaa-1111", "name_label": "web-01", "power_state": "Running"}`
)

// restRequest is one recorded request received by the fake server.
type restRequest struct {
	Method string
	Path   string
	Query  string
	Body   string
	Header http.Header
	Cookie string
}

// fakeXO is a fake Xen Orchestra REST server. It requires the SDK auth cookie
// and records every request so tests can assert on method, path, query,
// headers and body.
type fakeXO struct {
	*httptest.Server
	requests []restRequest
}

func newFakeXO(t *testing.T) *fakeXO {
	t.Helper()
	s := &fakeXO{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		cookie, _ := r.Cookie("authenticationToken")
		if cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}

		s.requests = append(s.requests, restRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Body:   string(body),
			Header: r.Header.Clone(),
			Cookie: cookie.Value,
		})

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Fake", "rest")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/vms":
			_, _ = fmt.Fprint(w, fixtureVMS)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/vms/aaaa-1111":
			_, _ = fmt.Fprint(w, fixtureVM)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/v0/plain":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprint(w, "hello world")
		case r.Method == http.MethodPost && r.URL.Path == "/rest/v0/vms":
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"id": "cccc-3333", "name_label": "new-vm"}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/rest/v0/vms/aaaa-1111":
			_, _ = fmt.Fprint(w, `{"id": "aaaa-1111", "name_label": "renamed", "power_state": "Running"}`)
		case r.Method == http.MethodPut && r.URL.Path == "/rest/v0/vms/aaaa-1111":
			_, _ = fmt.Fprint(w, `{"id": "aaaa-1111", "name_label": "replaced", "power_state": "Halted"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/v0/vms/aaaa-1111":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"message":"not found"}`)
		}
	}))
	return s
}

func (s *fakeXO) last() restRequest {
	return s.requests[len(s.requests)-1]
}

func isolate(t *testing.T, url string) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE", "XOA_YES"} {
		t.Setenv(key, "")
	}
	t.Setenv("XOA_ENDPOINT", url)
	t.Setenv("XOA_TOKEN", "test-token")
}

func newTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "xo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String(cli.FlagProfile, "", "")
	root.PersistentFlags().String(cli.FlagOutput, "table", "")
	root.PersistentFlags().Bool(cli.FlagJSON, false, "")
	root.AddCommand(NewCommand())
	return root
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runWithStdin(t, nil, args...)
}

// runWithStdin runs the command with an explicit stdin reader (for --data -).
func runWithStdin(t *testing.T, stdin io.Reader, args ...string) (string, error) {
	t.Helper()
	root := newTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	if stdin != nil {
		root.SetIn(stdin)
	}
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

// --- get --------------------------------------------------------------------

func TestRestGetTable(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	out, err := run(t, "rest", "get", "vms")
	if err != nil {
		t.Fatalf("rest get vms: %v", err)
	}
	for _, expected := range []string{"id", "name_label", "power_state", "web-01", "db-01", "Running"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
	req := server.last()
	if req.Method != http.MethodGet || req.Path != "/rest/v0/vms" {
		t.Fatalf("unexpected request: %s %s", req.Method, req.Path)
	}
	if req.Cookie != "test-token" {
		t.Fatalf("expected the SDK auth cookie, got %q", req.Cookie)
	}
}

func TestRestGetJSON(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	out, err := run(t, "rest", "get", "vms", "--output", "json")
	if err != nil {
		t.Fatalf("rest get --output json: %v", err)
	}
	var vms []map[string]any
	if err := json.Unmarshal([]byte(out), &vms); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(vms) != 2 || vms[0]["name_label"] != "web-01" {
		t.Fatalf("unexpected payload: %s", out)
	}
}

func TestRestGetYAML(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	out, err := run(t, "rest", "get", "vms/aaaa-1111", "--output", "yaml")
	if err != nil {
		t.Fatalf("rest get --output yaml: %v", err)
	}
	if !strings.Contains(out, "name_label: web-01") {
		t.Fatalf("unexpected YAML payload: %s", out)
	}
}

func TestRestGetQuery(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	out, err := run(t, "rest", "get", "vms", "--query", "[].name_label")
	if err != nil {
		t.Fatalf("rest get --query: %v", err)
	}
	if strings.TrimSpace(out) != "web-01\ndb-01" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestRestGetQueryParam(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	if _, err := run(t, "rest", "get", "vms", "--param", "limit=10", "--param", "filter=type:VM"); err != nil {
		t.Fatalf("rest get --param: %v", err)
	}
	if server.last().Query != "filter=type%3AVM&limit=10" {
		t.Fatalf("unexpected query string: %q", server.last().Query)
	}
}

func TestRestGetInvalidParam(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	if _, err := run(t, "rest", "get", "vms", "--param", "no-equals-sign"); err == nil {
		t.Fatal("expected an error for a malformed --param")
	}
}

func TestRestGetHeader(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	if _, err := run(t, "rest", "get", "vms", "--header", "X-Test: foo"); err != nil {
		t.Fatalf("rest get --header: %v", err)
	}
	if got := server.last().Header.Get("X-Test"); got != "foo" {
		t.Fatalf("expected X-Test: foo, got %q", got)
	}
}

func TestRestGetPlainBody(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	out, err := run(t, "rest", "get", "plain")
	if err != nil {
		t.Fatalf("rest get plain: %v", err)
	}
	if strings.TrimSpace(out) != "hello world" {
		t.Fatalf("unexpected plain output: %q", out)
	}
}

func TestRestGetInclude(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	out, err := run(t, "rest", "get", "vms", "-i")
	if err != nil {
		t.Fatalf("rest get -i: %v", err)
	}
	// The status line and the response headers are emitted alongside the body
	// (both go through the command writer, stderr on the real CLI).
	if !strings.Contains(out, "HTTP/1.1 200 OK") || !strings.Contains(out, "X-Fake: rest") {
		t.Fatalf("expected status line and headers in output:\n%s", out)
	}
	if !strings.Contains(out, "web-01") {
		t.Fatalf("expected the body to be rendered too:\n%s", out)
	}
}

func TestRestGetAPIError(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	_, err := run(t, "rest", "get", "nope")
	if err == nil {
		t.Fatal("expected an error when the API returns 404")
	}
	if !strings.Contains(err.Error(), "rest get nope") || !strings.Contains(err.Error(), "404") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- post / put / patch ------------------------------------------------------

func TestRestPostData(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	out, err := run(t, "rest", "post", "vms", "--data", `{"name_label":"new-vm","template":"aaaa-1111"}`, "--output", "json")
	if err != nil {
		t.Fatalf("rest post --data: %v", err)
	}
	var created map[string]any
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if created["id"] != "cccc-3333" {
		t.Fatalf("unexpected payload: %s", out)
	}

	req := server.last()
	if req.Method != http.MethodPost || req.Path != "/rest/v0/vms" {
		t.Fatalf("unexpected request: %s %s", req.Method, req.Path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("request body is not JSON: %v\n%s", err, req.Body)
	}
	if body["name_label"] != "new-vm" || body["template"] != "aaaa-1111" {
		t.Fatalf("unexpected request body: %s", req.Body)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected a JSON content type, got %q", got)
	}
}

func TestRestPostDataFromStdin(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	stdin := strings.NewReader(`{"name_label":"from-stdin"}`)
	if _, err := runWithStdin(t, stdin, "rest", "post", "vms", "--data", "-"); err != nil {
		t.Fatalf("rest post --data -: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(server.last().Body), &body); err != nil {
		t.Fatalf("request body is not JSON: %v\n%s", err, server.last().Body)
	}
	if body["name_label"] != "from-stdin" {
		t.Fatalf("unexpected request body: %s", server.last().Body)
	}
}

func TestRestPatchData(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	if _, err := run(t, "rest", "patch", "vms/aaaa-1111", "--data", `{"name_label":"renamed"}`); err != nil {
		t.Fatalf("rest patch --data: %v", err)
	}
	req := server.last()
	if req.Method != http.MethodPatch || req.Path != "/rest/v0/vms/aaaa-1111" {
		t.Fatalf("unexpected request: %s %s", req.Method, req.Path)
	}
}

func TestRestPutData(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	if _, err := run(t, "rest", "put", "vms/aaaa-1111", "--data", `{"name_label":"replaced"}`); err != nil {
		t.Fatalf("rest put --data: %v", err)
	}
	req := server.last()
	if req.Method != http.MethodPut || req.Path != "/rest/v0/vms/aaaa-1111" {
		t.Fatalf("unexpected request: %s %s", req.Method, req.Path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("request body is not JSON: %v\n%s", err, req.Body)
	}
	if body["name_label"] != "replaced" {
		t.Fatalf("unexpected request body: %s", req.Body)
	}
}

func TestRestInvalidData(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	if _, err := run(t, "rest", "post", "vms", "--data", "not-json"); err == nil {
		t.Fatal("expected an error for invalid --data")
	} else if !strings.Contains(err.Error(), "invalid --data") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(server.requests) != 0 {
		t.Fatal("no request must be sent when the body is invalid")
	}
}

func TestRestDataWithGetRejected(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	_, err := run(t, "rest", "get", "vms", "--data", `{"a":1}`)
	if err == nil {
		t.Fatal("expected an error when --data is used with get")
	}
	if len(server.requests) != 0 {
		t.Fatal("no request must be sent when the flags are invalid")
	}
}

// --- delete -----------------------------------------------------------------

func TestRestDeleteYes(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	if _, err := run(t, "rest", "delete", "vms/aaaa-1111", "--yes"); err != nil {
		t.Fatalf("rest delete --yes: %v", err)
	}
	req := server.last()
	if req.Method != http.MethodDelete || req.Path != "/rest/v0/vms/aaaa-1111" {
		t.Fatalf("unexpected request: %s %s", req.Method, req.Path)
	}
	if req.Body != "" {
		t.Fatalf("delete must not carry a body, got %q", req.Body)
	}
}

func TestRestDeleteRequiresConfirmation(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	_, err := run(t, "rest", "delete", "vms/aaaa-1111")
	if err == nil {
		t.Fatal("expected an error when no confirmation is given and stdin is not a terminal")
	}
	if !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(server.requests) != 0 {
		t.Fatal("the delete must not be executed without confirmation")
	}
}

// XOA_YES is the script counterpart of --yes for rest delete.

func TestRestDeleteSkipsConfirmationWithEnvYes(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)
	t.Setenv("XOA_YES", "1")

	if _, err := run(t, "rest", "delete", "vms/aaaa-1111"); err != nil {
		t.Fatalf("rest delete with XOA_YES=1: %v", err)
	}
	req := server.last()
	if req.Method != http.MethodDelete || req.Path != "/rest/v0/vms/aaaa-1111" {
		t.Fatalf("unexpected request: %s %s", req.Method, req.Path)
	}
}

// --- argument validation ----------------------------------------------------

func TestRestInvalidMethod(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	_, err := run(t, "rest", "head", "vms")
	if err == nil {
		t.Fatal("expected an error for an unsupported method")
	}
	if !strings.Contains(err.Error(), "unsupported method") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(server.requests) != 0 {
		t.Fatal("no request must be sent for an unsupported method")
	}
}

func TestRestMethodIsCaseInsensitive(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolate(t, server.URL)

	if _, err := run(t, "rest", "GET", "vms"); err != nil {
		t.Fatalf("rest GET (uppercase) should be accepted: %v", err)
	}
	if server.last().Method != http.MethodGet {
		t.Fatalf("expected a GET request, got %s", server.last().Method)
	}
}

func TestRestNoCredentials(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE"} {
		t.Setenv(key, "")
	}

	_, err := run(t, "rest", "get", "vms")
	if err == nil {
		t.Fatal("expected an error when nothing is configured")
	}
	if !strings.Contains(err.Error(), "no endpoint configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}
