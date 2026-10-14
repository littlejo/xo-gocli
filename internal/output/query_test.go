package output

import (
	"strings"
	"testing"
)

// awsBackticks keeps JSON backtick literals and turns the AWS CLI's non-JSON
// ones into raw strings. These tests pin the translation (including its
// interaction with quoted strings) so that a regression cannot silently
// change the meaning of a --query expression.

func TestAwsBackticksBasic(t *testing.T) {
	for in, want := range map[string]string{
		"[?power_state==`Running`].name_label": "[?power_state=='Running'].name_label",
		"`web-01`":                             "'web-01'",
		"``":                                   "''",
		// JSON literals are kept: they are booleans, numbers, JSON strings.
		"[?ha_enabled==`true`].name_label": "[?ha_enabled==`true`].name_label",
		"[?CPUs.number==`2`].id":           "[?CPUs.number==`2`].id",
		"[?name_label==`\"web-01\"`]":      "[?name_label==`\"web-01\"`]",
		"`null`":                           "`null`",
		// A quoted identifier is data too.
		"\"a`b\"": "\"a`b\"",
	} {
		if got := awsBackticks(in); got != want {
			t.Errorf("awsBackticks(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAwsBackticksKeepsSingleQuotedStringsUntouched(t *testing.T) {
	// A backtick inside a single-quoted literal is data, not a delimiter.
	in := "'a`b'"
	if got := awsBackticks(in); got != in {
		t.Fatalf("awsBackticks(%q) = %q, want unchanged", in, got)
	}
}

func TestAwsBackticksQuoting(t *testing.T) {
	// A single quote inside a non-JSON literal is escaped. (Double quotes
	// would make it an identifier, a field name, not a string.)
	if got := awsBackticks("[?name==`it's`]"); got != `[?name=='it\'s']` {
		t.Fatalf("unexpected re-quoting: %q", got)
	}
	// A literal without quotes keeps single quotes.
	if got := awsBackticks("`hello world`"); got != "'hello world'" {
		t.Fatalf("unexpected quoting: %q", got)
	}
}

func TestAwsBackticksUnclosedIsLeftAlone(t *testing.T) {
	// An unterminated backtick is not a literal: it must pass through so the
	// JMESPath compiler rejects the expression with a real syntax error
	// instead of the CLI corrupting the expression.
	in := "`unclosed"
	if got := awsBackticks(in); got != in {
		t.Fatalf("awsBackticks(%q) = %q, want unchanged", in, got)
	}
	if _, err := Query(in, []map[string]any{{"a": 1}}); err == nil {
		t.Fatal("an unterminated backtick must fail to compile")
	}
}

func TestQueryNilData(t *testing.T) {
	// A query against a nil payload (e.g. an endpoint that returned nothing)
	// must not panic and must yield an empty, present result.
	result, err := Query("[].name_label", nil)
	if err != nil {
		t.Fatalf("Query on nil: %v", err)
	}
	if result == nil || !result.Present {
		t.Fatalf("expected a present result, got %+v", result)
	}
	if result.Value != nil {
		t.Fatalf("expected a nil value, got %#v", result.Value)
	}
}

func TestRenderUnsupportedFormat(t *testing.T) {
	var buf strings.Builder
	if err := Render(&buf, Format("csv"), Table{}, nil, nil); err == nil {
		t.Fatal("Render(csv): expected an error")
	}
}

func TestRenderValueScalarFallbacks(t *testing.T) {
	// A query projecting a number or a nested object must not lose data:
	// scalars print as-is, composite values fall back to compact JSON.
	data := []map[string]any{
		{"name_label": "web-01", "memory": map[string]any{"size": 2147483648.0}},
	}
	cases := []struct {
		query  string
		expect string
	}{
		{"[0].memory.size", "2147483648"},
		{"[0].memory", `"size": 2147483648`},
		{"[0].name_label", "web-01"},
	}
	for _, tc := range cases {
		var buf strings.Builder
		query, err := Query(tc.query, data)
		if err != nil {
			t.Fatalf("Query(%q): %v", tc.query, err)
		}
		if err := Render(&buf, FormatText, Table{}, nil, query); err != nil {
			t.Fatalf("Render(%q): %v", tc.query, err)
		}
		if got := strings.TrimSpace(buf.String()); !strings.Contains(got, tc.expect) {
			t.Errorf("Render(%q) = %q, want it to contain %q", tc.query, got, tc.expect)
		}
	}
}

func TestRenderTextSingleObject(t *testing.T) {
	var buf strings.Builder
	raw, err := Normalize(map[string]any{"name_label": "web-01", "power_state": "Running"})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if err := Render(&buf, FormatText, Table{}, raw, nil); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	for _, expected := range []string{"name_label: web-01", "power_state: Running"} {
		if !strings.Contains(out, expected) {
			t.Errorf("text output missing %q:\n%s", expected, out)
		}
	}
}

func TestRenderTextScalar(t *testing.T) {
	var buf strings.Builder
	if err := Render(&buf, FormatText, Table{}, "just-a-string", nil); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "just-a-string" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestNormalizeInvalid(t *testing.T) {
	// A channel cannot be JSON-encoded and must surface a clean error rather
	// than a panic.
	if _, err := Normalize(make(chan int)); err == nil {
		t.Fatal("Normalize(chan): expected an error")
	}
}
