package taskwait

import (
	"errors"
	"strings"
	"testing"
)

func TestDefaultNotFoundIsStatusBased(t *testing.T) {
	// A genuine 404 is reported concisely.
	err := defaultNotFound("abc", errors.New(`API error: 404 Not Found - {"message":"not found"}`))
	if !strings.Contains(err.Error(), `task "abc" not found`) {
		t.Fatalf("expected the concise not-found message: %v", err)
	}

	// A 500 whose body merely contains "404" is not a not-found.
	err = defaultNotFound("abc", errors.New(`API error: 500 Internal Server Error - {"message":"inner 404"}`))
	if strings.Contains(err.Error(), "not found") {
		t.Fatalf("a 500 mentioning 404 in its body must not be 'not found': %v", err)
	}
	if !strings.Contains(err.Error(), `cannot get task "abc"`) {
		t.Fatalf("the cannot-get form is expected: %v", err)
	}
}
