package output

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
)

type normBase struct {
	Labels []string `json:"labels,omitempty"`
}

type normItem struct {
	normBase
	Name     string            `json:"name"`
	Tags     []string          `json:"tags,omitempty"`
	Nil      []string          `json:"nil_slice"`
	Meta     map[string]string `json:"meta,omitempty"`
	Children []normItem        `json:"children,omitempty"`
	Raw      []byte            `json:"raw,omitempty"`
	When     time.Time         `json:"when"`
	Count    int               `json:"count,omitempty"`
	Skipped  []string          `json:"-"`
	hidden   []string
}

// TestNormalizeKeepsEmptyCollections pins that empty slices and maps are
// present in the normalized data (as [] and {}), the way the Xen Orchestra
// API returns them, whether encoding/json dropped them (omitempty) or wrote
// null; while empty scalars stay omitted and the field rules of
// encoding/json (tag names, "-", embedding, Marshaler types, []byte) hold.
func TestNormalizeKeepsEmptyCollections(t *testing.T) {
	got, err := Normalize([]normItem{{Name: "a", Tags: []string{"x"}, Children: []normItem{{Name: "child"}}, hidden: []string{"secret"}}})
	if err != nil {
		t.Fatal(err)
	}
	items := got.([]any)
	item := items[0].(map[string]any)

	for key, want := range map[string]any{
		"tags":      []any{"x"},
		"labels":    []any{}, // from the embedded struct
		"nil_slice": []any{}, // null in plain encoding/json
		"meta":      map[string]any{},
		"name":      "a",
	} {
		if !reflect.DeepEqual(item[key], want) {
			t.Errorf("%s = %#v, want %#v", key, item[key], want)
		}
	}
	for _, key := range []string{"raw", "count", "Skipped", "hidden"} {
		if _, ok := item[key]; ok {
			t.Errorf("%s should be omitted, got %#v", key, item[key])
		}
	}
	if _, ok := item["when"].(string); !ok {
		t.Errorf("when should stay a JSON string (time.Time is a Marshaler), got %#v", item["when"])
	}
	child := item["children"].([]any)[0].(map[string]any)
	if !reflect.DeepEqual(child["tags"], []any{}) || !reflect.DeepEqual(child["children"], []any{}) {
		t.Errorf("nested collections should be kept, got %#v", child)
	}

	// A nil slice at the top level is an empty list, not null.
	var none []normItem
	if got, _ := Normalize(none); !reflect.DeepEqual(got, []any{}) {
		t.Errorf("Normalize(nil slice) = %#v, want []", got)
	}
}

// TestQueryOnUntaggedVMs is the user-facing case: --query with contains()
// on a list where some VMs have no tags used to fail with
// "invalid type for: <nil>".
func TestQueryOnUntaggedVMs(t *testing.T) {
	vms := []payloads.VM{
		{NameLabel: "web-01", Tags: []string{"production"}},
		{NameLabel: "scratch"},
	}
	res, err := Query("[?contains(tags, 'production')].name_label", vms)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Value, []any{"web-01"}) {
		t.Fatalf("got %#v, want [web-01]", res.Value)
	}
	res, err = Query("[?length(tags) == `0`].name_label", vms)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Value, []any{"scratch"}) {
		t.Fatalf("got %#v, want [scratch]", res.Value)
	}
}

// TestQueryBacktickLiterals pins the AWS CLI meaning of backtick literals:
// JSON values compare as their type, other text as a string.
func TestQueryBacktickLiterals(t *testing.T) {
	var data any
	if err := json.Unmarshal([]byte(`[
		{"name": "a", "ha": true,  "cpus": 2, "state": "Running", "label": "it's"},
		{"name": "b", "ha": false, "cpus": 4, "state": "Halted",  "label": "other"}
	]`), &data); err != nil {
		t.Fatal(err)
	}
	for expr, want := range map[string][]any{
		"[?ha==`true`].name":          {"a"},
		"[?cpus==`2`].name":           {"a"},
		"[?cpus > `2`].name":          {"b"},
		"[?state==`Running`].name":    {"a"},
		"[?state==`\"Halted\"`].name": {"b"},
		"[?label==`it's`].name":       {"a"},
	} {
		res, err := Query(expr, data)
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		if !reflect.DeepEqual(res.Value, want) {
			t.Errorf("%s = %#v, want %#v", expr, res.Value, want)
		}
	}
}

type normPtrBase struct {
	Groups []string `json:"groups,omitempty"`
}

type normEmbedded struct {
	*normPtrBase
	Name string `json:"name"`
}

// TestNormalizeFollowsEmbeddingRules pins that only the fields encoding/json
// itself emits get their empty collections restored: the exported fields of
// an unexported embedded struct are promoted (by value, see normItem, and by
// non-nil pointer), while a nil embedded pointer contributes no field at all.
func TestNormalizeFollowsEmbeddingRules(t *testing.T) {
	got, err := Normalize(normEmbedded{normPtrBase: &normPtrBase{}, Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if obj := got.(map[string]any); !reflect.DeepEqual(obj["groups"], []any{}) {
		t.Errorf("a field promoted through a non-nil embedded pointer should be kept, got %#v", obj)
	}

	got, err = Normalize(normEmbedded{Name: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if obj := got.(map[string]any); len(obj) != 1 || obj["name"] != "b" {
		t.Errorf("a nil embedded pointer must not add fields, got %#v", obj)
	}
}
