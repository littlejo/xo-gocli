package task

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

func newGetCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a task",
		Long: `Get a task from Xen Orchestra, as a detailed, human readable view.

The task is referenced by its id, as returned by 'xo task list' or by an
asynchronous operation (for example 'xo vm start').

The human (default) view is a detail sheet: the status, the operation (type
and name), when it started and ended, its duration, the user that ran it and
the object it targeted. For a failed task the error (code and message) is
shown; for a successful one with a result, that result is shown.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo task get <id> --output json' is unchanged.

Examples:
  xo task get a1b2c3d4e5f6
  xo task get <id> --output json
  xo task get <id> --query 'result'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if id == "" || strings.Contains(id, "/") {
				return fmt.Errorf("invalid task id %q", id)
			}
			if err := output.ValidateQuery(query); err != nil {
				return err
			}
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}

			cfg, err := config.Load(cli.ProfileName(cmd))
			if err != nil {
				return err
			}

			httpClient, err := cli.NewHTTPClient(cmd, cfg)
			if err != nil {
				return err
			}

			var task map[string]any
			endpoint := tasksEndpoint + "/" + id
			if err := client.TypedGet(cmd.Context(), httpClient, endpoint, listParams{Fields: "*"}, &task); err != nil {
				return taskNotFound(id, err, cfg.Insecure)
			}

			// The structured formats (json/yaml/text) and --query are served
			// straight from the raw object, exactly as before; only the default
			// human view is the new detail sheet.
			if format != output.FormatTable || query != "" {
				return renderTaskRaw(cmd.OutOrStdout(), format, task, query)
			}
			return renderTaskDetail(cmd.OutOrStdout(), task)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'result'")
	return cmd
}

// taskNotFound delegates to cli.NotFound, which reports a 404 concisely and
// keeps the raw API error as debug detail.
func taskNotFound(id string, err error, insecure bool) error {
	return cli.NotFound("task", "get", id, err, insecure)
}

// renderTaskRaw renders the task in the structured formats (json/yaml/text) or
// as a --query projection. It emits the raw object so machine readable output
// and the query pipeline are unchanged.
func renderTaskRaw(w io.Writer, format output.Format, t map[string]any, query string) error {
	if query != "" {
		queryResult, err := output.Query(query, t)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, t, queryResult)
	}

	normalized, err := output.Normalize(t)
	if err != nil {
		return err
	}
	return output.Render(w, format, output.Table{}, normalized, nil)
}

// renderTaskDetail renders the single task as a human readable detail sheet.
// It is a plain view (tasks are served as a raw map with no relationships to
// resolve): status, operation, timing, user and target, plus the error for a
// failed task or the result for a successful one.
func renderTaskDetail(w io.Writer, t map[string]any) error {
	lines := make([]string, 0, 14)
	props, _ := t["properties"].(map[string]any)

	// Header: id and (status).
	id := strField(t, "id")
	header := "Task " + id
	if status := strField(t, "status"); status != "" {
		header += "  (" + status + ")"
	}
	lines = append(lines, header)

	// Operation
	if ty := strField(props, "type"); ty != "" {
		lines = append(lines, output.DetailField("Type", ty))
	}
	if nm := strField(props, "name"); nm != "" {
		lines = append(lines, output.DetailField("Name", nm))
	}
	if m := strField(props, "method"); m != "" {
		lines = append(lines, output.DetailField("Method", m))
	}
	if u := strField(props, "userId"); u != "" {
		lines = append(lines, output.DetailField("User", u))
	}
	if o := strField(props, "objectId"); o != "" {
		lines = append(lines, output.DetailField("Target", o))
	}

	// Timing
	if s := timeText(t["start"]); s != "" {
		lines = append(lines, output.DetailField("Started", s))
	}
	if e := timeText(t["end"]); e != "" {
		lines = append(lines, output.DetailField("Ended", e))
	}
	if d := durationText(t); d != "" {
		lines = append(lines, output.DetailField("Duration", d))
	}
	if subtasks, ok := t["tasks"].([]any); ok && len(subtasks) > 0 {
		lines = append(lines, output.DetailField("Subtasks", fmt.Sprintf("%d", len(subtasks))))
	}

	// Result: the error for a failed task, or the result of a successful one.
	if msg := messageText(t); msg != "" {
		if strField(t, "status") == "failure" {
			code := ""
			if result, ok := t["result"].(map[string]any); ok {
				code = strField(result, "code")
			}
			if code != "" {
				lines = append(lines, output.DetailField("Error", code+" — "+msg))
			} else {
				lines = append(lines, output.DetailField("Error", msg))
			}
		} else {
			lines = append(lines, output.DetailField("Result", msg))
		}
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// durationText computes "ended - started" as a human duration, or "" when
// either bound is missing or unparseable.
func durationText(t map[string]any) string {
	start, ok1 := parseTime(t["start"])
	end, ok2 := parseTime(t["end"])
	if !ok1 || !ok2 {
		return ""
	}
	if end.Before(start) {
		return ""
	}
	return end.Sub(start).Round(time.Second).String()
}

// parseTime turns the API's start/end field (an RFC3339 string, or an integer
// of seconds or milliseconds) into a time.Time.
func parseTime(value any) (time.Time, bool) {
	switch v := value.(type) {
	case string:
		parsed, err := time.Parse(time.RFC3339, v)
		return parsed, err == nil
	case float64:
		return time.UnixMilli(int64(v)), true
	case int64:
		return time.UnixMilli(v), true
	default:
		return time.Time{}, false
	}
}

// messageText extracts a human readable message from the task result, which
// may be an object with a "message" field or a plain string.
func messageText(t map[string]any) string {
	if s, ok := t["result"].(string); ok {
		return s
	}
	result, ok := t["result"].(map[string]any)
	if !ok {
		return ""
	}
	return strField(result, "message")
}
