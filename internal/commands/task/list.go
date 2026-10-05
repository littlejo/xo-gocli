package task

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagQuery  = "query"
	flagLimit  = "limit"
	flagStatus = "status"
)

// tasksEndpoint is the REST resource holding asynchronous tasks.
const tasksEndpoint = "tasks"

// listParams is the query string for GET /tasks, mirroring what the SDK's own
// task service sends (fields=* asks the server for the full objects).
type listParams struct {
	Fields string `json:"fields,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Filter string `json:"filter,omitempty"`
}

func newListCommand() *cobra.Command {
	var (
		query  string
		limit  int
		status string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List asynchronous tasks",
		Long: `List asynchronous tasks from Xen Orchestra.

Tasks are returned by most asynchronous operations (VM actions,
provisioning, ...). Each task has a status: pending, success, failure or
interrupted.

Examples:
  xo task list
  xo task list --output json
  xo task list --query '[].id'
  xo task list --status failure
  xo task list --limit 10`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}
			if limit < 0 {
				return fmt.Errorf("--limit must be greater than or equal to 0")
			}
			if err := output.ValidateQuery(query); err != nil {
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

			params := listParams{Fields: "*"}
			if limit > 0 {
				params.Limit = limit
			}
			if status != "" {
				params.Filter = "status:" + status
			}

			var tasks []map[string]any
			if err := client.TypedGet(cmd.Context(), httpClient, tasksEndpoint, params, &tasks); err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot list tasks: %v", err), cfg.Insecure)
			}

			return renderTasks(cmd.OutOrStdout(), format, tasks, query)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. '[].id'")
	flags.IntVar(&limit, flagLimit, 0, "maximum number of tasks to return (0 for no limit)")
	flags.StringVar(&status, flagStatus, "", "filter by status: pending, success, failure, interrupted")

	return cmd
}

// renderTasks applies the optional --query expression and renders the result
// in the requested format. The raw objects are plain maps (the REST shape) so
// that --query and the structured formats see every field.
func renderTasks(w io.Writer, format output.Format, tasks []map[string]any, query string) error {
	queryResult, err := output.Query(query, tasks)
	if err != nil {
		return err
	}

	table := output.Table{
		Headers: []string{"ID", "STATUS", "TYPE", "NAME", "STARTED", "ENDED"},
		Empty:   "tasks",
	}
	for _, t := range tasks {
		props, _ := t["properties"].(map[string]any)
		table.Rows = append(table.Rows, []string{
			strField(t, "id"),
			strField(t, "status"),
			strField(props, "type"),
			strField(props, "name"),
			timeText(t["start"]),
			timeText(t["end"]),
		})
	}

	// For structured formats without a query, normalize so the output only
	// contains the requested data.
	var raw any = tasks
	if format != output.FormatTable && (queryResult == nil || !queryResult.Present) {
		normalized, err := output.Normalize(tasks)
		if err != nil {
			return err
		}
		raw = normalized
	}

	return output.Render(w, format, table, raw, queryResult)
}

// timeText renders a REST API time (RFC3339 string or Unix milliseconds) in a
// stable UTC form, or "" when absent (for example, a task that has not ended
// yet).
func timeText(value any) string {
	const layout = "2006-01-02T15:04:05Z"
	switch v := value.(type) {
	case string:
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return v
		}
		return parsed.UTC().Format(layout)
	case float64:
		return time.UnixMilli(int64(v)).UTC().Format(layout)
	case int64:
		return time.UnixMilli(v).UTC().Format(layout)
	default:
		return ""
	}
}

// strField returns a string field or "" when absent / not a string.
func strField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
