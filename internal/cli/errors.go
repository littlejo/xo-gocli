package cli

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// The SDK (and the Go standard library it builds on) reports transport and
// parsing failures with implementation detail: "context deadline exceeded
// (Client.Timeout …)", "dial tcp 127.0.0.1:8901: connect: connection
// refused", "json: cannot unmarshal …". Users should not need to understand
// those to diagnose a failure (AGENTS.md). translateSDKError maps the
// well-known forms to concise messages; the raw text is always kept so
// --debug / $XOA_DEBUG can reveal it.
//
// Only errors produced by the SDK transport and by JSON decoding of API
// responses are translated. Local errors (file I/O, flag validation, …) never
// match a pattern and pass through unchanged.

// currentClientTimeout is the timeout resolved by the most recent call to
// Timeout, which every command makes (via NewClient) before issuing a
// request. The translator uses it so "timed out after Xs" reports the value
// that was actually in effect, not the 30 s default.
var currentClientTimeout = defaultClientTimeout

// apiStatusPattern matches the status code in the SDK's
// "API error: 404 Not Found - <body>" form (and the status-only
// "API error: 404" form produced by the raw REST helpers).
var apiStatusPattern = regexp.MustCompile(`API error: (\d{3})`)

// translateSDKError returns a concise replacement for msg when it matches a
// known SDK/Go error form, along with the original message so the caller can
// keep it as a debug-only detail. The CLI context that precedes the SDK error
// ("cannot list VMs: …") is preserved; only the SDK part is replaced. When
// nothing matches it returns ("", msg) — the message is already
// user-presentable (an "API error: 500 …" line with the server's body, a TLS
// certificate error, a local I/O error, …).
func translateSDKError(msg string) (translated, raw string) {
	tail := msg
	if i := sdkErrorStart(msg); i >= 0 {
		tail = msg[i:]
	}
	replacement, ok := translateSDKErrorTail(tail)
	if !ok {
		return "", msg
	}
	// Keep the CLI context ("cannot list VMs: …") before the translated part.
	return msg[:len(msg)-len(tail)] + replacement, msg
}

// sdkErrorMarkers are the beginnings of the error messages the SDK produces
// (see v2/client/client.go and internal/common/core). Everything before the
// first marker is CLI context.
var sdkErrorMarkers = []string{
	"failed to do request ",
	"failed to send request: ",
	"failed to make request ",
	"failed to unmarshal response ",
	"failed to marshal response ",
	"failed to read response body ",
	"failed to authenticate: ",
	"API error: ",
	"json: cannot unmarshal",
}

func sdkErrorStart(msg string) int {
	best := -1
	for _, marker := range sdkErrorMarkers {
		if i := strings.Index(msg, marker); i >= 0 && (best == -1 || i < best) {
			best = i
		}
	}
	return best
}

// translateSDKErrorTail maps one SDK/Go error message (without any CLI
// prefix) to a concise replacement, reporting whether it recognized the form.
func translateSDKErrorTail(tail string) (string, bool) {
	if strings.Contains(tail, "context deadline exceeded") {
		return fmt.Sprintf("the request timed out after %s (raise it with --timeout)", currentClientTimeout), true
	}
	if i := strings.Index(tail, "dial tcp "); i >= 0 {
		return unreachableEndpoint(tail[i:]), true
	}
	if strings.Contains(tail, "failed to unmarshal response") || strings.HasPrefix(tail, "json: cannot unmarshal") {
		return "the Xen Orchestra API returned a malformed response", true
	}
	switch apiStatusText(tail) {
	case http.StatusUnauthorized:
		return "authentication was rejected by the server (401)", true
	case http.StatusForbidden:
		return "the server rejected the request (403): the credentials lack the required permission", true
	}
	return "", false
}

// unreachableEndpoint turns "dial tcp 127.0.0.1:8901: connect: connection
// refused" (and the i/o-timeout variant) into "cannot reach <addr>: <reason>".
func unreachableEndpoint(s string) string {
	// s is "dial tcp <addr>: <…>: <reason>"; the address is the first token
	// after "dial tcp ", the reason the text after the last ": ".
	fields := strings.SplitN(s, " ", 3)
	if len(fields) < 3 {
		return s
	}
	rest := fields[2]
	reason := rest
	if j := strings.LastIndex(rest, ": "); j >= 0 {
		reason = rest[j+len(": "):]
	}
	addr := rest
	if k := strings.Index(rest, ": "); k >= 0 {
		addr = rest[:k]
	}
	return fmt.Sprintf("cannot reach %s: %s", addr, reason)
}

// APIStatus returns the HTTP status code carried by an SDK error in the
// "API error: <status> …" form, or 0 when the error carries none.
//
// It is the status-based replacement for matching the "404" substring
// anywhere in an error string: the body of a 500 response may itself contain
// "404", which the old substring check misreported as "not found".
func APIStatus(err error) int {
	if err == nil {
		return 0
	}
	return apiStatusText(err.Error())
}

func apiStatusText(msg string) int {
	m := apiStatusPattern.FindStringSubmatch(msg)
	if m == nil {
		return 0
	}
	status, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return status
}
