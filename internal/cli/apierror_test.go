package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestAPIErrorKeepsTheChain(t *testing.T) {
	cause := fmt.Errorf("failed to do request: %w", context.Canceled)
	err := APIError(cause, false, "cannot list %s", "VMs")
	if !strings.HasPrefix(err.Error(), "cannot list VMs: ") {
		t.Fatalf("message = %q", err.Error())
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(%v, context.Canceled) = false", err)
	}
}

func TestAPIErrorSameMessageAsInsecureHint(t *testing.T) {
	for _, cause := range []error{
		errors.New("API error: 500 Internal Server Error - boom"),
		errors.New("tls: failed to verify certificate: x509: certificate signed by unknown authority"),
		errors.New(`failed to do request: Get "https://xo/rest/v0/vms": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`),
	} {
		want := InsecureHint(fmt.Sprintf("cannot list VMs: %v", cause), false)
		got := APIError(cause, false, "cannot list VMs")
		if got.Error() != want.Error() || Detail(got) != Detail(want) {
			t.Fatalf("APIError(%v) = %q / %q, want %q / %q", cause, got, Detail(got), want, Detail(want))
		}
		if !errors.Is(got, cause) {
			t.Fatalf("APIError(%v) lost its cause", cause)
		}
	}
}

func TestNotFoundKeepsTheChain(t *testing.T) {
	cause := errors.New("API error: 404 Not Found - not found")
	err := NotFound("VM", "get", "abc", cause, false)
	if err.Error() != `VM "abc" not found` {
		t.Fatalf("message = %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatal("NotFound lost its cause")
	}
}
