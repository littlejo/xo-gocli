package vm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"
)

// xva.go holds the raw REST plumbing shared by 'xo vm export' and
// 'xo vm import'.
//
// The SDK v2 typed VM service does not expose XVA/OVA import/export yet (a
// known SDK gap, tracked in docs/internals/sdk-v2.md), so these commands reach the
// REST endpoints through the SDK's exported *client.Client — the same single
// API boundary as the rest of the CLI: the authentication cookie, the base
// URL and the TLS handling all come from the SDK. The missing typed
// operations should be contributed to the SDK.
//
// Endpoints (per the XO REST API):
//
//	export: GET  /vms/{id}.{format}   (format: xva | ova, ?compress=true)
//	import: POST /pools/{pool}/vms    (body: raw XVA archive, ?sr=<sr-id>)

// xvRequest builds and sends an authenticated request against the XO REST API
// using the SDK v2 client's exported HTTP facilities. endpoint is relative to
// the client's base URL (/rest/v0). contentLength may be -1 when unknown
// (e.g. when the body is read from stdin).
func xvRequest(ctx context.Context, c *client.Client, method, endpoint string, query url.Values, body io.Reader, contentType string, contentLength int64) (*http.Response, error) {
	u := *c.BaseURL
	u.Path = path.Join(c.BaseURL.Path, endpoint)
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if contentLength >= 0 {
		req.ContentLength = contentLength
	}
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: string(c.AuthToken)})

	return c.HttpClient.Do(req)
}

// xvaAPIError turns a non-2xx response into the concise "API error: ..."
// form used across the CLI (the caller still closes resp.Body).
func xvaAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("API error: %s - %s", resp.Status, strings.TrimSpace(string(body)))
}
