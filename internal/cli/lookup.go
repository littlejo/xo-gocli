package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gofrs/uuid"
)

// LookupID turns the reference a user typed into an object id: ref itself
// when it is a UUID (no request), otherwise the id of the one object whose
// name_label is exactly ref. kind names the resource in messages ("VM").
//
// The name lookup costs one list request. list receives an API filter that
// narrows the result on the server (the XO filter syntax matches substrings,
// so the exact, case-sensitive comparison is done here); it receives "" when
// the name cannot be quoted safely, and the whole collection is compared.
// No match is reported as not found, saying it was looked up as a name;
// several matches list their ids.
func LookupID[T any](ctx context.Context, kind, ref string, insecure bool,
	list func(ctx context.Context, filter string) ([]T, error),
	idOf func(T) uuid.UUID, nameOf func(T) string,
) (uuid.UUID, error) {
	if id, err := uuid.FromString(ref); err == nil {
		return id, nil
	}
	if strings.TrimSpace(ref) == "" {
		return uuid.UUID{}, fmt.Errorf("invalid %s reference %q (expected a UUID or a name)", kind, ref)
	}

	filter := ""
	if !strings.ContainsAny(ref, `"\`) {
		filter = fmt.Sprintf("name_label:%q", ref)
	}
	objects, err := list(ctx, filter)
	if err != nil {
		return uuid.UUID{}, APIError(err, insecure, "cannot look up %s %q", kind, ref)
	}

	var ids []string
	var match uuid.UUID
	for _, o := range objects {
		if nameOf(o) == ref {
			match = idOf(o)
			ids = append(ids, match.String())
		}
	}
	switch len(ids) {
	case 0:
		return uuid.UUID{}, fmt.Errorf("%s %q not found (no %s has this name)", kind, ref, kind)
	case 1:
		return match, nil
	default:
		sort.Strings(ids)
		return uuid.UUID{}, fmt.Errorf("%d %ss are named %q, use an id: %s", len(ids), kind, ref, strings.Join(ids, ", "))
	}
}
