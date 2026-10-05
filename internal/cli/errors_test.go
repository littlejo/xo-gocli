package cli

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// setClientTimeout makes Timeout report (and remember) the given duration,
// without a cobra command: it uses the $XOA_TIMEOUT path.
func setClientTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	t.Setenv(EnvTimeout, d.String())
	if got := Timeout(nil); got != d {
		t.Fatalf("Timeout = %v, want %v", got, d)
	}
}

// rawTimeout is the SDK error a request produces when the HTTP client times
// out (reproduced against a slow server).
const rawTimeout = `cannot list VMs: failed to do request Get "http://127.0.0.1:3001/rest/v0/vms?fields=%2A": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`

// rawDialRefused is the SDK error for an unreachable endpoint.
const rawDialRefused = `cannot list VMs: failed to do request Get "http://127.0.0.1:8901/rest/v0/vms?fields=%2A": dial tcp 127.0.0.1:8901: connect: connection refused`

// rawUnmarshal is the SDK error when the API response does not match the
// payload model (here a task "result" that is a string, a documented XO quirk).
const rawUnmarshal = `cannot get task "abc": failed to unmarshal response json: cannot unmarshal string into Go struct field Task.result of type payloads.Result`

// rawBadUUID is the SDK error when a field that must hold a UUID receives
// something else (the "bad id in an edge case" form).
const rawBadUUID = `cannot list VMs: failed to unmarshal response uuid: incorrect UUID length 4 in string "vbd1"`

func TestTranslateTimeout(t *testing.T) {
	setClientTimeout(t, 30*time.Second)
	got, raw := translateSDKError(rawTimeout)
	if raw != rawTimeout {
		t.Fatalf("the raw message must be returned for the debug detail, got: %q", raw)
	}
	if !strings.Contains(got, "the request timed out after 30s") {
		t.Fatalf("timeout not translated: %q", got)
	}
	if !strings.Contains(got, "cannot list VMs:") {
		t.Fatalf("the CLI context must be preserved: %q", got)
	}
	for _, internal := range []string{"context deadline", "Client.Timeout", "failed to do request"} {
		if strings.Contains(got, internal) {
			t.Fatalf("Go internals leaked into the message %q: %q", got, internal)
		}
	}
}

func TestTranslateTimeoutUsesEffectiveDuration(t *testing.T) {
	setClientTimeout(t, 90*time.Second)
	got, _ := translateSDKError(rawTimeout)
	if !strings.Contains(got, "timed out after 1m30s") {
		t.Fatalf("the effective --timeout must be reported, got: %q", got)
	}
}

func TestTranslateUnreachable(t *testing.T) {
	got, raw := translateSDKError(rawDialRefused)
	if raw != rawDialRefused {
		t.Fatalf("raw mismatch: %q", raw)
	}
	if !strings.Contains(got, "cannot reach 127.0.0.1:8901: connection refused") {
		t.Fatalf("dial error not translated: %q", got)
	}
	if strings.Contains(got, "dial tcp") {
		t.Fatalf("Go internals leaked: %q", got)
	}
}

func TestTranslateUnreachableIOTimeout(t *testing.T) {
	raw := `failed to do request Get "http://host:8443/rest/v0/vms": dial tcp 10.0.0.5:8443: i/o timeout`
	got, _ := translateSDKError(raw)
	if !strings.Contains(got, "cannot reach 10.0.0.5:8443: i/o timeout") {
		t.Fatalf("dial i/o timeout not translated: %q", got)
	}
}

func TestTranslateMalformedResponse(t *testing.T) {
	for _, raw := range []string{rawUnmarshal, rawBadUUID} {
		got, kept := translateSDKError(raw)
		if kept != raw {
			t.Fatalf("raw mismatch: %q", kept)
		}
		if !strings.Contains(got, "the Xen Orchestra API returned a malformed response") {
			t.Fatalf("unmarshal error not translated: %q", got)
		}
		for _, internal := range []string{"cannot unmarshal", "uuid:", "failed to unmarshal", "Go struct field"} {
			if strings.Contains(got, internal) {
				t.Fatalf("Go internals leaked into %q: %q", got, internal)
			}
		}
	}
}

func TestTranslate401(t *testing.T) {
	raw := `cannot list VMs: API error: 401 Unauthorized - {"message":"unauthorized"}`
	got, kept := translateSDKError(raw)
	if kept != raw {
		t.Fatalf("raw mismatch: %q", kept)
	}
	if !strings.Contains(got, "authentication was rejected by the server (401)") {
		t.Fatalf("401 not translated: %q", got)
	}
	if strings.Contains(got, "API error") {
		t.Fatalf("the raw API form must not remain: %q", got)
	}
}

func TestTranslate403(t *testing.T) {
	raw := `cannot get pool "abc": API error: 403 Forbidden - {"message":"access denied"}`
	got, _ := translateSDKError(raw)
	if !strings.Contains(got, "the server rejected the request (403)") {
		t.Fatalf("403 not translated: %q", got)
	}
}

func TestTranslateLeavesUnrecognizedErrorsAlone(t *testing.T) {
	// A 500 with the server's own body is already informative: keep it.
	plain := `cannot list VMs: API error: 500 Internal Server Error - {"message":"boom"}`
	if got, _ := translateSDKError(plain); got != "" {
		t.Fatalf("a 5xx with a body must not be rewritten, got: %q", got)
	}
	// TLS errors must stay untouched so the --insecure hint still applies.
	tls := `cannot list VMs: failed to do request Get "https://xoa.test/rest/v0/vms": tls: x509: certificate signed by unknown authority`
	if got, _ := translateSDKError(tls); got != "" {
		t.Fatalf("TLS errors must not be rewritten, got: %q", got)
	}
	// Local (non-SDK) errors must stay untouched.
	local := `cannot read "web-01.xva": open web-01.xva: no such file or directory`
	if got, _ := translateSDKError(local); got != "" {
		t.Fatalf("local errors must not be rewritten, got: %q", got)
	}
	// A Ctrl+C abort is left as-is: it is neither a failure class to rewrite
	// nor a leak.
	canceled := `cannot list VMs: failed to do request Get "https://xoa.test/rest/v0/vms": interrupt signal received`
	if got, _ := translateSDKError(canceled); got != "" {
		t.Fatalf("a cancellation must not be rewritten, got: %q", got)
	}
}

func TestAPIStatus(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{errors.New(`API error: 404 Not Found - {"message":"not found"}`), http.StatusNotFound},
		{errors.New(`API error: 500 Internal Server Error - {"message":"inner 404"}`), http.StatusInternalServerError},
		{errors.New(`API error: 403 Forbidden`), http.StatusForbidden},
		{errors.New(`cannot get host "h": API error: 401 Unauthorized - x`), http.StatusUnauthorized},
		{errors.New(`failed to do request Get "https://x": dial tcp 1.2.3.4:443: connect: connection refused`), 0},
		{errors.New(`json: cannot unmarshal`), 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := APIStatus(c.err); got != c.want {
			t.Errorf("APIStatus(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}

// The 404 detection must key off the actual HTTP status: a 500 whose body
// merely contains "404" must not be reported as "not found".
func TestNotFoundKeysOffStatusNotSubstring(t *testing.T) {
	// Regression: the body of the 500 response contains "404".
	err := NotFound("host", "get", "abc", errors.New(`API error: 500 Internal Server Error - {"message":"inner 404"}`), false)
	if strings.Contains(err.Error(), "not found") {
		t.Fatalf("a 500 mentioning 404 in its body must not be 'not found': %v", err)
	}
	if !strings.Contains(err.Error(), `cannot get host "abc"`) {
		t.Fatalf("the cannot-get form is expected: %v", err)
	}

	// A genuine 404 stays concise and keeps the raw error as detail.
	err = NotFound("host", "get", "abc", errors.New(`API error: 404 Not Found - {"message":"object not found"}`), false)
	if !strings.Contains(err.Error(), `host "abc" not found`) {
		t.Fatalf("expected the concise not-found message: %v", err)
	}
	if Detail(err) == "" {
		t.Fatal("the 404 must keep the raw error as a debug detail")
	}
}

// A non-404 lookup failure is translated like any other SDK error, and the
// raw text is kept as the debug detail.
func TestNotFoundTranslatesNon404(t *testing.T) {
	setClientTimeout(t, 30*time.Second)
	err := NotFound("VM", "get", "abc", errors.New(`failed to do request Get "https://xoa.test/rest/v0/vms/abc": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`), false)
	if !strings.Contains(err.Error(), `cannot get VM "abc"`) {
		t.Fatalf("the cannot-get form is expected: %v", err)
	}
	if !strings.Contains(err.Error(), "the request timed out after 30s") {
		t.Fatalf("the timeout must be translated: %v", err)
	}
	if strings.Contains(err.Error(), "context deadline") {
		t.Fatalf("Go internals must not leak: %v", err)
	}
	if d := Detail(err); !strings.Contains(d, "context deadline exceeded") {
		t.Fatalf("the raw error must be kept as the debug detail, got: %q", d)
	}
}

// InsecureHint translates recognized SDK errors (keeping the raw text as the
// debug detail) and still appends the --insecure hint for TLS errors.
func TestInsecureHintTranslation(t *testing.T) {
	err := InsecureHint(rawDialRefused, false)
	if strings.Contains(err.Error(), "dial tcp") {
		t.Fatalf("dial internals must not leak: %v", err)
	}
	if !strings.Contains(err.Error(), "cannot reach 127.0.0.1:8901: connection refused") {
		t.Fatalf("dial error not translated: %v", err)
	}
	if d := Detail(err); !strings.Contains(d, "dial tcp 127.0.0.1:8901") {
		t.Fatalf("the raw error must be kept as the debug detail, got: %q", d)
	}

	// TLS: no translation, hint still appended.
	tls := InsecureHint(`failed to do request Get "https://xoa.test/rest/v0/vms": tls: x509: certificate signed by unknown authority`, false)
	if !strings.Contains(tls.Error(), "--insecure") {
		t.Fatalf("the --insecure hint must be kept for TLS errors: %v", tls)
	}
	if strings.Contains(tls.Error(), "cannot reach") {
		t.Fatalf("a TLS error must not be rewritten as unreachable: %v", tls)
	}

	// Unrecognized: returned as-is, no hidden detail.
	plain := InsecureHint(`API error: 500 Internal Server Error - {"message":"boom"}`, false)
	if Detail(plain) != "" {
		t.Fatalf("untranslated errors must not carry a hidden detail: %q", Detail(plain))
	}
}

// --debug must reveal the raw SDK error behind a translated message.
func TestExecuteRevealsRawErrorBehindTranslation(t *testing.T) {
	isolateDebugEnv(t)
	t.Setenv(EnvDebug, "1")
	err := InsecureHint(rawDialRefused, false)
	root := newDebugTestRoot(err)

	var errOut strings.Builder
	root.SetErr(&errOut)
	if err := Execute(context.Background(), root, []string{"fail"}); err == nil {
		t.Fatal("expected the injected error")
	}
	got := errOut.String()
	if !strings.Contains(got, "dial tcp 127.0.0.1:8901") {
		t.Errorf("$XOA_DEBUG must reveal the raw error:\n%s", got)
	}
	if !strings.Contains(err.Error(), "cannot reach 127.0.0.1:8901: connection refused") {
		t.Errorf("the concise message must still be the error: %v", err)
	}
}
