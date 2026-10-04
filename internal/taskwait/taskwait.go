// Package taskwait polls a task until it reaches a terminal state and renders
// it in the CLI output format. It is shared by the 'xo task wait' command and
// the --wait flag of the asynchronous actions ('xo vm start --wait',
// 'xo vbd connect --wait', ...).
//
// The polling deliberately does not use the SDK's typed Task().Wait: that
// unmarshals into payloads.Task, whose Result is a struct — but XO sometimes
// returns a task "result" as a plain string (a documented XO quirk), so the
// typed wait fails to unmarshal such tasks, and it does not treat
// "interrupted" as terminal. Instead we poll with the SDK's own REST client
// (client.TypedGet), the same single API boundary as 'xo task get'.
package taskwait

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

// PollInterval is how often a still-pending task is re-fetched. It matches the
// SDK's own task.Wait polling cadence (2 s). It is a variable (not a const) so
// tests can shorten it.
var PollInterval = 2 * time.Second

// tasksEndpoint is the REST resource holding asynchronous tasks.
const tasksEndpoint = "tasks"

// terminalStatuses are the task statuses that will not change any further; the
// wait stops as soon as a task reaches one of them.
var terminalStatuses = map[string]bool{
	"success":     true,
	"failure":     true,
	"interrupted": true,
}

// taskParams mirrors what the SDK's own task service sends (fields=* asks the
// server for the full objects).
type taskParams struct {
	Fields string `json:"fields,omitempty"`
}

// Options configures a Wait call.
type Options struct {
	// Out receives the rendered completed task.
	Out io.Writer
	// Stderr receives a one-line progress hint when the task does not complete
	// on the first poll (nil = silent).
	Stderr io.Writer
	// ID is the id of the task to wait for.
	ID string
	// Format is the output format used to render the completed task.
	Format output.Format
	// Query is an optional JMESPath expression applied to the completed task.
	Query string
	// Deadline bounds the wait; zero (or negative) means wait until the task
	// completes or the context is cancelled.
	Deadline time.Duration
	// NotFound, when non-nil, converts a failed task fetch (e.g. a 404) into a
	// caller-specific error; when nil a generic one is returned.
	NotFound func(id string, err error) error
}

// Wait blocks until the task reaches a terminal state (success, failure or
// interrupted), then renders it in the given format and returns an error
// describing the outcome: nil on success, non-nil on failure, interruption,
// the deadline being reached, or the context being cancelled.
//
// The completed task is always rendered to Options.Out; a non-successful
// outcome is reported through the returned error so the exit status reflects
// it.
func Wait(ctx context.Context, httpClient *client.Client, o Options) error {
	var cancel context.CancelFunc
	if o.Deadline > 0 {
		ctx, cancel = context.WithTimeout(ctx, o.Deadline)
		defer cancel()
	}

	noted := false
	for {
		var task map[string]any
		err := client.TypedGet(ctx, httpClient, tasksEndpoint+"/"+o.ID, taskParams{Fields: "*"}, &task)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				// The poll itself was cut off because the wait ended
				// (deadline hit or cancellation).
				return endedError(o.ID, o.Deadline, ctxErr)
			}
			if o.NotFound != nil {
				return o.NotFound(o.ID, err)
			}
			return defaultNotFound(o.ID, err)
		}

		status := strField(task, "status")
		if terminalStatuses[status] {
			// The task has finished: show it, then let the exit status reflect
			// the outcome (non-zero on failure / interruption).
			if err := Render(o.Out, o.Format, task, o.Query); err != nil {
				return err
			}
			if status != "success" {
				return outcomeError(o.ID, status, task)
			}
			return nil
		}

		// Still pending: announce the wait once, then wait for the next poll
		// while honoring the context.
		if !noted && o.Stderr != nil {
			if _, err := fmt.Fprintf(o.Stderr, "Waiting for task %s to complete...\n", o.ID); err != nil {
				return err
			}
			noted = true
		}
		select {
		case <-ctx.Done():
			return endedError(o.ID, o.Deadline, ctx.Err())
		case <-time.After(PollInterval):
		}
	}
}

// Render renders a single task in the requested format. The human format uses
// a single-row table with the 'xo task list' columns plus a message column (the
// useful part of the result for failed tasks); the structured formats emit the
// normalized object (or its --query projection).
func Render(w io.Writer, format output.Format, t map[string]any, query string) error {
	if format == output.FormatTable && query == "" {
		props, _ := t["properties"].(map[string]any)
		table := output.Table{
			Headers: []string{"ID", "STATUS", "TYPE", "NAME", "STARTED", "ENDED", "MESSAGE"},
			Rows: [][]string{{
				strField(t, "id"),
				strField(t, "status"),
				strField(props, "type"),
				strField(props, "name"),
				timeText(t["start"]),
				timeText(t["end"]),
				messageText(t),
			}},
		}
		return output.Render(w, format, table, t, nil)
	}

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

// endedError turns an ended wait into a concise error. A deadline is reported
// as a timeout; a cancellation (Ctrl+C) as an interruption.
func endedError(id string, deadline time.Duration, ctxErr error) error {
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return fmt.Errorf("task %s did not complete within %s", id, deadline)
	}
	return fmt.Errorf("waiting for task %s was interrupted", id)
}

// outcomeError reports a completed task that did not succeed, carrying the
// task's own message when available (the useful part of a failed result).
func outcomeError(id, status string, task map[string]any) error {
	if status == "interrupted" {
		return fmt.Errorf("task %s was interrupted", id)
	}
	if msg := messageText(task); msg != "" {
		return fmt.Errorf("task %s failed: %s", id, msg)
	}
	return fmt.Errorf("task %s failed", id)
}

// defaultNotFound is the not-found mapping used when Options.NotFound is nil:
// a 404 becomes a concise "not found" error, anything else a concise transport
// error (callers that need --debug details or the insecure hint pass their own
// NotFound).
func defaultNotFound(id string, err error) error {
	if err != nil && cli.APIStatus(err) == http.StatusNotFound {
		return fmt.Errorf("task %q not found", id)
	}
	return cli.InsecureHint(fmt.Sprintf("cannot get task %q: %v", id, err), false)
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
