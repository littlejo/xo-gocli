// Package token implements the 'xo token' command group.
//
// Authentication tokens are not wrapped by an SDK v2 typed service. They are
// exposed by the REST API under /rest/v0/users/<id>/authentication_tokens
// (the "me" alias is answered by the server with a single 307 redirect to the
// user's own id, which the HTTP client follows automatically), so this command
// reaches them through the SDK's own REST client (cli.NewHTTPClient) — the same
// single API boundary, not a second HTTP client. The missing typed service
// should be contributed to the SDK.
//
// The token id is the secret itself (32 random bytes, base64url). Listing and
// getting therefore mask it by default; the full value is only revealed on an
// explicit request, and 'create' prints it once (it cannot be retrieved again).
package token

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"time"

	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"

	"github.com/littlejo/xo-gocli/internal/cli"
)

// tokensEndpoint is the REST resource holding the current user's
// authentication tokens. The "me" alias is redirected by the server to the
// user's own id (a single 307), which the HTTP client follows automatically.
const tokensEndpoint = "users/me/authentication_tokens"

// authCookieName is the cookie the SDK v2 client uses to carry the token.
const authCookieName = "authenticationToken"

// doTokensRequest performs a GET or POST on the tokens endpoint and returns
// the raw response body. The SDK client's BaseURL is already suffixed with
// /rest/v0; the endpoint is joined to it and the auth cookie is attached.
// Redirects (the /users/me 307) are followed by the client, preserving the
// method and body for POST (307 semantics).
func doTokensRequest(ctx context.Context, c *client.Client, method, endpoint string, body []byte, params map[string]any) ([]byte, error) {
	u := *c.BaseURL
	u.Path = path.Join(c.BaseURL.Path, endpoint)
	if len(params) > 0 {
		q := u.Query()
		for k, v := range params {
			q.Add(k, fmt.Sprintf("%v", v))
		}
		u.RawQuery = q.Encode()
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: string(c.AuthToken)})

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	respBody, err := io.ReadAll(resp.Body)
	if cerr := resp.Body.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, cli.ResponseError(resp, respBody)
	}
	return respBody, nil
}

// maskToken hides a token secret so that listing or getting a token never
// exposes the full value. The token id is the secret itself.
func maskToken(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8] + "…"
}

// maskTokens returns a copy of tokens with the secret (the token id) masked.
func maskTokens(tokens []map[string]any) []map[string]any {
	masked := make([]map[string]any, len(tokens))
	for i, t := range tokens {
		cp := make(map[string]any, len(t))
		for k, v := range t {
			cp[k] = v
		}
		if id, ok := cp["id"].(string); ok {
			cp["id"] = maskToken(id)
		}
		masked[i] = cp
	}
	return masked
}

// timeText renders a REST API time (Unix milliseconds) as UTC RFC3339, or ""
// when absent.
func timeText(value any) string {
	const layout = "2006-01-02T15:04:05Z"
	switch v := value.(type) {
	case float64:
		return time.UnixMilli(int64(v)).UTC().Format(layout)
	case int64:
		return time.UnixMilli(v).UTC().Format(layout)
	default:
		return ""
	}
}

// strField returns a string field or "" when absent / not a string.
func strField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// clientID extracts client.id from a token, or "" when absent.
func clientID(t map[string]any) string {
	c, ok := t["client"].(map[string]any)
	if !ok {
		return ""
	}
	return strField(c, "id")
}

// tokenRow builds one table row from a token, masking the secret.
func tokenRow(t map[string]any) []string {
	return []string{
		maskToken(strField(t, "id")),
		strField(t, "description"),
		timeText(t["created_at"]),
		timeText(t["expiration"]),
		clientID(t),
	}
}
