package token

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
	secretTokenID = "abcdefghijklmnopqrstuvwxyz0123456789ABCD"
	otherTokenID  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcd"
	fixtureUser   = "0529b688-f70c-40f1-bc5f-d230b2aa11ce"
)

const fixtureTokens = `[
	{
		"id": "abcdefghijklmnopqrstuvwxyz0123456789ABCD",
		"description": "ci pipeline",
		"created_at": 1788445204108,
		"expiration": 1804224004108,
		"user_id": "0529b688-f70c-40f1-bc5f-d230b2aa11ce"
	},
	{
		"id": "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcd",
		"created_at": 1790571960181,
		"expiration": 1796165999010,
		"client": {"id": "z15uawq1atg"},
		"user_id": "0529b688-f70c-40f1-bc5f-d230b2aa11ce"
	}
]`

// fakeXO reproduces the relevant Xen Orchestra REST behavior for
// authentication tokens:
//
//	GET  /rest/v0/users/me/authentication_tokens  -> 307 to /users/<id>/authentication_tokens
//	POST /rest/v0/users/me/authentication_tokens  -> 307 (body replayed) -> 201 {token}
//	GET  /rest/v0/users/<id>/authentication_tokens -> 200 [tokens]
type fakeXO struct {
	*httptest.Server
	t            *testing.T
	lastPOST     string
	lastGetQuery string
}

func newFakeXO(t *testing.T) *fakeXO {
	t.Helper()
	s := &fakeXO{t: t}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		if cookie, err := r.Cookie("authenticationToken"); err != nil || cookie.Value != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}

		switch {
		case r.URL.Path == "/rest/v0/users/me/authentication_tokens":
			// The server redirects the "me" alias to the user's own id,
			// preserving the query string (it rebuilds Location from
			// req.originalUrl).
			if r.Method == http.MethodPost {
				s.lastPOST = string(body)
			}
			loc := fmt.Sprintf("/rest/v0/users/%s/authentication_tokens", fixtureUser)
			if r.URL.RawQuery != "" {
				loc += "?" + r.URL.RawQuery
			}
			w.Header().Set("Location", loc)
			w.WriteHeader(http.StatusTemporaryRedirect)
			return

		case r.URL.Path == fmt.Sprintf("/rest/v0/users/%s/authentication_tokens", fixtureUser):
			switch r.Method {
			case http.MethodGet:
				s.lastGetQuery = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, fixtureTokens)
			case http.MethodPost:
				s.lastPOST = string(body)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprint(w, `{"token": {"id": "abcdefghijklmnopqrstuvwxyz0123456789ABCD", "description": "ci pipeline", "created_at": 1788445204108, "expiration": 1804224004108, "user_id": "0529b688-f70c-40f1-bc5f-d230b2aa11ce"}}`)
			default:
				http.NotFound(w, r)
			}
			return

		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func isolatePointers(t *testing.T, url string) {
	t.Helper()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE"} {
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
	root := newTestRoot()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

// --- list -------------------------------------------------------------------

func TestTokenListTableMasked(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := run(t, "token", "list")
	if err != nil {
		t.Fatalf("token list: %v", err)
	}
	for _, expected := range []string{"ID", "DESCRIPTION", "CREATED", "EXPIRES", "CLIENT", "ci pipeline", "z15uawq1atg", "abcdefgh…"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
	// The full secret must never appear in the masked output.
	if strings.Contains(out, secretTokenID) || strings.Contains(out, otherTokenID) {
		t.Errorf("token list must not reveal the secret:\n%s", out)
	}
}

func TestTokenListNoSecret(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := run(t, "token", "list", "--no-secret", "--output", "json")
	if err != nil {
		t.Fatalf("token list --no-secret: %v", err)
	}
	if !strings.Contains(out, secretTokenID) {
		t.Fatalf("expected the full token id with --no-secret:\n%s", out)
	}
}

func TestTokenListJSONMasked(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := run(t, "token", "list", "--output", "json")
	if err != nil {
		t.Fatalf("token list --output json: %v", err)
	}
	var tokens []map[string]any
	if err := json.Unmarshal([]byte(out), &tokens); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0]["id"] != "abcdefgh…" {
		t.Fatalf("expected masked id, got %v", tokens[0]["id"])
	}
	if tokens[0]["description"] != "ci pipeline" {
		t.Fatalf("unexpected payload: %s", out)
	}
}

func TestTokenListQuery(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := run(t, "token", "list", "--query", "[?description==`ci pipeline`].description", "--output", "text")
	if err != nil {
		t.Fatalf("token list --query: %v", err)
	}
	if strings.TrimSpace(out) != "ci pipeline" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestTokenListLimit(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := run(t, "token", "list", "--limit", "3"); err != nil {
		t.Fatalf("token list --limit: %v", err)
	}
	if server.lastGetQuery != "limit=3" {
		t.Errorf("expected limit=3 query, got %q", server.lastGetQuery)
	}
}

func TestTokenListAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"boom"}`)
	}))
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := run(t, "token", "list")
	if err == nil {
		t.Fatal("expected an error when the API fails")
	}
	if !strings.Contains(err.Error(), "cannot list tokens") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTokenListNoCredentials(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	t.Setenv("XOA_CONFIG_FILE", t.TempDir()+"/config")
	for _, key := range []string{"XOA_PROFILE", "XOA_ENDPOINT", "XOA_TOKEN", "XOA_USERNAME", "XOA_PASSWORD", "XOA_INSECURE"} {
		t.Setenv(key, "")
	}

	_, err := run(t, "token", "list")
	if err == nil {
		t.Fatal("expected an error when nothing is configured")
	}
	if !strings.Contains(err.Error(), "no endpoint configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- get --------------------------------------------------------------------

func TestTokenGetTable(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := run(t, "token", "get", secretTokenID)
	if err != nil {
		t.Fatalf("token get: %v", err)
	}
	for _, expected := range []string{"ID", "DESCRIPTION", "ci pipeline", "abcdefgh…"} {
		if !strings.Contains(out, expected) {
			t.Errorf("table output missing %q:\n%s", expected, out)
		}
	}
	if strings.Contains(out, secretTokenID) {
		t.Errorf("token get must not reveal the secret by default:\n%s", out)
	}
}

func TestTokenGetNoSecret(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := run(t, "token", "get", secretTokenID, "--no-secret", "--output", "json")
	if err != nil {
		t.Fatalf("token get --no-secret: %v", err)
	}
	if !strings.Contains(out, secretTokenID) {
		t.Fatalf("expected the full token id with --no-secret:\n%s", out)
	}
}

func TestTokenGetQuery(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := run(t, "token", "get", secretTokenID, "--query", "description")
	if err != nil {
		t.Fatalf("token get --query: %v", err)
	}
	if strings.TrimSpace(out) != "ci pipeline" {
		t.Fatalf("unexpected query output: %q", out)
	}
}

func TestTokenGetNotFound(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	_, err := run(t, "token", "get", "fffffffffffffffffffffffffffffffffffff")
	if err == nil {
		t.Fatal("expected an error when the token does not exist")
	}
	if !strings.Contains(err.Error(), `token "fffffffffffffffffffffffffffffffffffff" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTokenGetMaskedIDRejected(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	// A masked id (from the default list output) cannot resolve a token.
	if _, err := run(t, "token", "get", "abcdefgh…"); err == nil {
		t.Fatal("expected an error when a masked id is used")
	}
}

// --- create -----------------------------------------------------------------

func TestTokenCreate(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := run(t, "token", "create", "--description", "ci pipeline", "--expires-in", "30 days")
	if err != nil {
		t.Fatalf("token create: %v", err)
	}
	// The created token is printed in full: this is the only moment the
	// secret is available again.
	if !strings.Contains(out, secretTokenID) {
		t.Fatalf("token create must print the full token:\n%s", out)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(server.lastPOST), &body); err != nil {
		t.Fatalf("cannot parse request body: %v\n%s", err, server.lastPOST)
	}
	if body["description"] != "ci pipeline" || body["expiresIn"] != "30 days" {
		t.Fatalf("unexpected request body: %s", server.lastPOST)
	}
}

func TestTokenCreateJSON(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	out, err := run(t, "token", "create", "--output", "json")
	if err != nil {
		t.Fatalf("token create --output json: %v", err)
	}
	var token map[string]any
	if err := json.Unmarshal([]byte(out), &token); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if token["id"] != secretTokenID {
		t.Fatalf("unexpected token payload: %s", out)
	}
}

func TestTokenCreateClientID(t *testing.T) {
	server := newFakeXO(t)
	defer server.Close()
	isolatePointers(t, server.URL)

	if _, err := run(t, "token", "create", "--client-id", "my-cli"); err != nil {
		t.Fatalf("token create --client-id: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(server.lastPOST), &body); err != nil {
		t.Fatalf("cannot parse request body: %v\n%s", err, server.lastPOST)
	}
	client, ok := body["client"].(map[string]any)
	if !ok || client["id"] != "my-cli" {
		t.Fatalf("expected client.id=my-cli in body, got %s", server.lastPOST)
	}
}
