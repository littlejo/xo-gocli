package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderRawJSONKeepsTheObject(t *testing.T) {
	var buf bytes.Buffer
	vm := sampleData()[0]
	if err := RenderRaw(&buf, FormatJSON, vm, ""); err != nil {
		t.Fatalf("RenderRaw: %v", err)
	}
	for _, want := range []string{`"name_label": "web-01"`, `"memory"`} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("JSON output misses %s:\n%s", want, buf.String())
		}
	}
}

func TestRenderRawQuery(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderRaw(&buf, FormatTable, sampleData()[0], "name_label"); err != nil {
		t.Fatalf("RenderRaw: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "web-01" {
		t.Fatalf("query output = %q, want web-01", buf.String())
	}
}

func TestRenderRawBadQuery(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderRaw(&buf, FormatJSON, sampleData()[0], "[?"); err == nil {
		t.Fatal("expected an error for an invalid query")
	}
}
