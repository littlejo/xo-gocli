// Package output renders structured data in table, JSON, YAML or text
// formats and applies JMESPath queries.
//
// The pipeline is: SDK response -> structured data -> query/filter ->
// output formatter. Tables are only produced for the default, unqueried view;
// query results are rendered with the requested format.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/jmespath-community/go-jmespath"
	"gopkg.in/yaml.v3"
)

// Format is one of the supported output formats.
type Format string

const (
	// FormatTable is the default, human readable format.
	FormatTable Format = "table"
	// FormatJSON renders data as JSON.
	FormatJSON Format = "json"
	// FormatYAML renders data as YAML.
	FormatYAML Format = "yaml"
	// FormatText renders a minimal key/value representation.
	FormatText Format = "text"
)

// Formats lists the valid output formats.
var Formats = []string{string(FormatTable), string(FormatJSON), string(FormatYAML), string(FormatText)}

// ParseFormat validates a user provided format name.
func ParseFormat(name string) (Format, error) {
	switch Format(name) {
	case FormatTable, FormatJSON, FormatYAML, FormatText:
		return Format(name), nil
	default:
		return "", fmt.Errorf("unsupported --output format %q (expected one of: %s)", name, strings.Join(Formats, ", "))
	}
}

// Table is a simple in-memory table for human readable output.
type Table struct {
	Headers []string
	Rows    [][]string
	// Empty is the resource noun shown when there are no rows, e.g. "VMs"
	// renders "No VMs found." It is optional: when empty, an empty table
	// renders nothing (the previous behavior), so callers that do not know
	// their resource noun are unaffected.
	Empty string
}

// QueryResult carries the outcome of a --query expression. Present is false
// when no query was given, in which case the table is rendered instead.
type QueryResult struct {
	Present bool
	Value   any
}

// Render writes the result to w in the given format.
//
// When a query was provided, its result is rendered: JSON/YAML produce
// machine readable documents, table/text print one value per line.
// When no query was provided, raw is rendered: table shows the table,
// JSON/YAML show the full structured data, text shows a compact form.
func Render(w io.Writer, format Format, table Table, raw any, query *QueryResult) error {
	if query != nil && query.Present {
		switch format {
		case FormatJSON:
			return renderJSON(w, query.Value)
		case FormatYAML:
			return renderYAML(w, query.Value)
		case FormatText, FormatTable:
			return renderValue(w, query.Value)
		default:
			return fmt.Errorf("unsupported output format %q", format)
		}
	}

	switch format {
	case FormatTable:
		return RenderTable(w, table.Headers, table.Rows, table.Empty)
	case FormatJSON:
		return renderJSON(w, raw)
	case FormatYAML:
		return renderYAML(w, raw)
	case FormatText:
		normalized, err := Normalize(raw)
		if err != nil {
			return err
		}
		return renderText(w, normalized)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

// ValidateQuery checks that the expression compiles. An empty expression is
// valid.
func ValidateQuery(expression string) error {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil
	}
	if _, err := jmespath.Compile(awsBackticks(expression)); err != nil {
		return fmt.Errorf("invalid --query expression %q: %v", expression, err)
	}
	return nil
}

// Query applies an AWS CLI like JMESPath expression to data.
// An empty expression returns an absent QueryResult.
func Query(expression string, data any) (*QueryResult, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, nil
	}
	if data == nil {
		return &QueryResult{Present: true}, nil
	}
	expression = awsBackticks(expression)

	// JMESPath works on map[string]interface{} / []interface{} trees, so the
	// structured data is round-tripped through JSON first. This keeps the
	// SDK response types as the single source of data.
	normalized, err := Normalize(data)
	if err != nil {
		return nil, err
	}
	result, err := jmespath.Search(expression, normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid --query expression %q: %w", expression, err)
	}
	return &QueryResult{Present: true, Value: result}, nil
}

// awsBackticks converts AWS CLI style backtick literals (`Running`) into the
// single quoted literals ('Running') of the JMESPath engine, so that the
// familiar AWS syntax works out of the box.
func awsBackticks(expression string) string {
	var b strings.Builder
	inString := false
	for i := 0; i < len(expression); i++ {
		c := expression[i]
		switch {
		case c == '\'':
			inString = !inString
			b.WriteByte(c)
		case c == '`' && !inString:
			j := i + 1
			for j < len(expression) && expression[j] != '`' {
				j++
			}
			if j >= len(expression) {
				b.WriteByte(c)
				continue
			}
			literal := expression[i+1 : j]
			if !strings.ContainsRune(literal, '\'') {
				b.WriteByte('\'')
				b.WriteString(literal)
				b.WriteByte('\'')
			} else {
				b.WriteByte('"')
				b.WriteString(literal)
				b.WriteByte('"')
			}
			i = j
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// Normalize converts any value into JSON-compatible primitives so that
// JMESPath and the structured renderers can process SDK types.
func Normalize(data any) (any, error) {
	if data == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("cannot process data: %w", err)
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, fmt.Errorf("cannot process data: %w", err)
	}
	return decoded, nil
}

// RenderTable writes a plain aligned table to w. When there are no rows and
// an empty message noun is given (e.g. "VMs"), it prints the header plus a
// "No VMs found." line instead of nothing: a silent no-op is a bad first-run
// experience, while machine output (json/yaml) still emits the empty
// structure it is supposed to.
func RenderTable(w io.Writer, headers []string, rows [][]string, empty ...string) error {
	emptyMsg := ""
	if len(empty) > 0 {
		emptyMsg = empty[0]
	}
	if len(rows) == 0 {
		if emptyMsg == "" {
			return nil
		}
		_, err := fmt.Fprintf(w, "%s\n\nNo %s found.\n", strings.Join(headers, "  "), emptyMsg)
		return err
	}
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	const columnGap = 2
	writeRow := func(cells []string) error {
		parts := make([]string, 0, len(cells))
		for i, cell := range cells {
			if i < len(widths) && i < len(cells)-1 {
				parts = append(parts, cell+strings.Repeat(" ", widths[i]-len(cell)+columnGap))
			} else {
				parts = append(parts, cell)
			}
		}
		_, err := fmt.Fprintln(w, strings.TrimRight(strings.Join(parts, ""), " "))
		return err
	}
	if err := writeRow(headers); err != nil {
		return err
	}
	separator := make([]string, len(headers))
	for i, width := range widths {
		separator[i] = strings.Repeat("-", width)
	}
	if err := writeRow(separator); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeRow(row); err != nil {
			return err
		}
	}
	return nil
}

func renderJSON(w io.Writer, data any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func renderYAML(w io.Writer, data any) error {
	encoded, err := yaml.Marshal(intifyNumbers(data))
	if err != nil {
		return err
	}
	_, err = w.Write(encoded)
	return err
}

// intifyNumbers rewrites JSON numbers in a decoded tree (map[string]any /
// []any) into typed Go values before YAML marshaling.
//
// Data reaching the renderer has been round-tripped through json.Unmarshal,
// so every number is a float64; yaml.v3 would then print whole values of 10
// digits or more in exponent form (size: 2.147483648e+09). Converting whole
// values to int64 keeps them exact and plain. The same walk also normalizes
// json.Number values (a raw payload decoded with UseNumber), which yaml
// would render as quoted strings, into numbers. Values that do not fit in an
// int64, or that carry a fractional part, stay float64.
func intifyNumbers(v any) any {
	switch value := v.(type) {
	case map[string]any:
		for key, item := range value {
			value[key] = intifyNumbers(item)
		}
		return value
	case []any:
		for i, item := range value {
			value[i] = intifyNumbers(item)
		}
		return value
	case float64:
		if !math.IsNaN(value) && !math.IsInf(value, 0) && value == math.Trunc(value) &&
			value >= math.MinInt64 && value <= math.MaxInt64 {
			return int64(value)
		}
		return value
	case json.Number:
		if i, err := value.Int64(); err == nil {
			return i
		}
		if f, err := value.Float64(); err == nil {
			return f
		}
		return string(value)
	default:
		return v
	}
}

// renderValue is the plain rendering used for table and text formats:
// strings and lists of strings are printed one per line, anything else is
// printed as compact JSON so information is never lost.
func renderValue(w io.Writer, data any) error {
	switch value := data.(type) {
	case nil:
		return nil
	case string:
		_, err := fmt.Fprintln(w, value)
		return err
	case []any:
		allStrings := true
		for _, item := range value {
			if _, ok := item.(string); !ok {
				allStrings = false
				break
			}
		}
		if allStrings {
			for _, item := range value {
				if _, err := fmt.Fprintln(w, item.(string)); err != nil {
					return err
				}
			}
			return nil
		}
		return renderJSON(w, value)
	default:
		return renderJSON(w, value)
	}
}

// renderText is a compact human readable rendering of raw structured data:
// a list of objects becomes a table with one column per key, a single
// object becomes key/value lines, scalars are printed as-is.
func renderText(w io.Writer, data any) error {
	list, ok := data.([]any)
	if !ok {
		switch value := data.(type) {
		case nil:
			return nil
		case map[string]any:
			keys := make([]string, 0, len(value))
			for key := range value {
				keys = append(keys, key)
			}
			sortStrings(keys)
			for _, key := range keys {
				if _, err := fmt.Fprintf(w, "%s: %s\n", key, scalar(value[key])); err != nil {
					return err
				}
			}
			return nil
		default:
			_, err := fmt.Fprintln(w, scalar(data))
			return err
		}
	}

	// A list that does not contain objects (for example the list of
	// resource URIs returned by a REST index endpoint such as
	// /rest/v0/vms) cannot be turned into a column table. Render one
	// value per line so nothing is dropped.
	if !listHasObjects(list) {
		for _, item := range list {
			if _, err := fmt.Fprintln(w, scalar(item)); err != nil {
				return err
			}
		}
		return nil
	}

	rows, headers := genericTable(list)
	return RenderTable(w, headers, rows)
}

// listHasObjects reports whether list contains at least one object, i.e.
// whether it can be rendered as a column table.
func listHasObjects(list []any) bool {
	for _, item := range list {
		if _, ok := item.(map[string]any); ok {
			return true
		}
	}
	return false
}

// genericTable builds a table from a list of objects, using the union of the
// object keys (in order of first appearance) as columns.
func genericTable(list []any) ([][]string, []string) {
	var headers []string
	index := map[string]int{}
	for _, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for key := range obj {
			if _, known := index[key]; !known {
				index[key] = len(headers)
				headers = append(headers, key)
			}
		}
	}
	sortStrings(headers)

	rows := make([][]string, 0, len(list))
	for _, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := make([]string, len(headers))
		for i, header := range headers {
			if value, found := obj[header]; found {
				row[i] = scalar(value)
			}
		}
		rows = append(rows, row)
	}
	return rows, headers
}

// scalar renders a single value for key/value output.
func scalar(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(encoded)
	}
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
