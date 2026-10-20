package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gofrs/uuid"
)

type named struct {
	id   uuid.UUID
	name string
}

func lookup(t *testing.T, ref string, objects []named, listErr error) (uuid.UUID, string, error) {
	t.Helper()
	var filter string
	called := false
	id, err := LookupID(context.Background(), "VM", ref, false,
		func(_ context.Context, f string) ([]named, error) {
			called, filter = true, f
			return objects, listErr
		},
		func(o named) uuid.UUID { return o.id },
		func(o named) string { return o.name })
	if !called {
		filter = "<no request>"
	}
	return id, filter, err
}

var (
	idA = uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440001"))
	idB = uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440002"))
)

func TestLookupIDUUIDNeedsNoRequest(t *testing.T) {
	id, filter, err := lookup(t, idA.String(), nil, nil)
	if err != nil || id != idA || filter != "<no request>" {
		t.Fatalf("LookupID(uuid) = %v, %q, %v", id, filter, err)
	}
}

func TestLookupIDExactName(t *testing.T) {
	// The server filter matches substrings: web-01 and web-010 both come back.
	objects := []named{{idA, "web-01"}, {idB, "web-010"}}
	id, filter, err := lookup(t, "web-01", objects, nil)
	if err != nil || id != idA {
		t.Fatalf("LookupID(web-01) = %v, %v", id, err)
	}
	if filter != `name_label:"web-01"` {
		t.Fatalf("filter = %q", filter)
	}
}

func TestLookupIDErrors(t *testing.T) {
	if _, _, err := lookup(t, "db", []named{{idA, "db-01"}}, nil); err == nil || err.Error() != `VM "db" not found` {
		t.Fatalf("no match: %v", err)
	}
	_, _, err := lookup(t, "web", []named{{idB, "web"}, {idA, "web"}}, nil)
	if err == nil || err.Error() != `2 VMs are named "web", use an id: `+idA.String()+", "+idB.String() {
		t.Fatalf("ambiguous: %v", err)
	}
	cause := errors.New("API error: 500 Internal Server Error - boom")
	if _, _, err := lookup(t, "web", nil, cause); err == nil || !errors.Is(err, cause) || !strings.HasPrefix(err.Error(), `cannot look up VM "web": `) {
		t.Fatalf("list failure: %v", err)
	}
	if _, _, err := lookup(t, " ", nil, nil); err == nil || !strings.Contains(err.Error(), "expected a UUID or a name") {
		t.Fatalf("blank: %v", err)
	}
}

func TestLookupIDUnquotableNameListsEverything(t *testing.T) {
	id, filter, err := lookup(t, `say "hi"`, []named{{idA, `say "hi"`}}, nil)
	if err != nil || id != idA || filter != "" {
		t.Fatalf("LookupID = %v, %q, %v", id, filter, err)
	}
}
