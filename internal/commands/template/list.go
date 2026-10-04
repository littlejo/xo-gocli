package template

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/go-units"
	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

const (
	flagQuery = "query"
	flagLimit = "limit"

	// templatesEndpoint is the REST resource that holds VM templates. The SDK
	// v2 does not (yet) expose a typed service for it, so the CLI reaches it
	// through the SDK's own REST client (cli.NewHTTPClient) — the same single
	// API boundary, not a second HTTP client. The missing operation should be
	// contributed to the SDK.
	templatesEndpoint = "vm-templates"
)

// listParams is the query string for GET /vm-templates. fields=* asks the
// server for the full objects, as the SDK's own services do.
type listParams struct {
	Fields string `json:"fields,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

func newListCommand() *cobra.Command {
	var (
		query string
		limit int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List VM templates",
		Long: `List VM templates from Xen Orchestra.

Templates are first-class objects in Xen Orchestra (REST resource
"vm-templates"); unlike ordinary VMs they are not part of the "vms"
collection, which is why 'xo vm list' does not show them.

Examples:
  xo template list
  xo template list --output json
  xo template list --query '[].name_label'
  xo template list --limit 10`,
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

			var templates []map[string]any
			if err := client.TypedGet(cmd.Context(), httpClient, templatesEndpoint, params, &templates); err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot list templates: %v", err), cfg.Insecure)
			}

			// Names (instead of raw UUIDs) are only shown in the human table;
			// --output json/yaml/text and --query keep the raw references, so
			// the resolver — and its extra batch requests — is only built for
			// the table.
			var resolver *resolve.Client
			if format == output.FormatTable && query == "" {
				resolver, err = cli.NewResolver(cmd, cfg)
				if err != nil {
					return err
				}
			}
			return renderTemplates(cmd.OutOrStdout(), cmd.Context(), format, templates, query, resolver)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. '[].name_label'")
	flags.IntVar(&limit, flagLimit, 0, "maximum number of templates to return (0 for no limit)")

	return cmd
}

// renderTemplates applies the optional --query expression and renders the
// result in the requested format. The human table shows the pool each template
// belongs to by name (one batch lookup for every row, never one lookup per
// template); the raw objects are plain maps (the REST shape) so that --query
// and the structured formats see every field, unchanged.
func renderTemplates(w io.Writer, ctx context.Context, format output.Format, templates []map[string]any, query string, r *resolve.Client) error {
	queryResult, err := output.Query(query, templates)
	if err != nil {
		return err
	}

	var poolIDs []uuid.UUID
	poolByTemplate := make([]uuid.UUID, len(templates))
	for i, t := range templates {
		id, err := uuid.FromString(strField(t, "$pool"))
		if err != nil {
			id = uuid.UUID{}
		}
		poolByTemplate[i] = id
		if !id.IsNil() {
			poolIDs = append(poolIDs, id)
		}
	}
	poolNames := map[string]string{}
	if r != nil && len(poolIDs) > 0 {
		poolNames = r.PoolBatchNames(ctx, poolIDs)
	}

	table := output.Table{
		Headers: []string{"ID", "NAME", "DEFAULT", "MEMORY", "CPUS", "POOL"},
	}
	for i, t := range templates {
		pool := strField(t, "$pool")
		if id := poolByTemplate[i]; !id.IsNil() {
			if n, ok := poolNames[id.String()]; ok {
				pool = n
			} else {
				pool = id.String()
			}
		}
		table.Rows = append(table.Rows, []string{
			strField(t, "id"),
			strField(t, "name_label"),
			fmt.Sprintf("%t", boolField(t, "isDefaultTemplate")),
			memoryText(t),
			fmt.Sprintf("%d", nestedInt(t, "CPUs", "number")),
			pool,
		})
	}

	// For structured formats without a query, normalize so the output only
	// contains the requested data.
	var raw any = templates
	if format != output.FormatTable && (queryResult == nil || !queryResult.Present) {
		normalized, err := output.Normalize(templates)
		if err != nil {
			return err
		}
		raw = normalized
	}

	return output.Render(w, format, table, raw, queryResult)
}

// memoryText renders the template's memory size in a human readable form.
func memoryText(t map[string]any) string {
	size := int64(nestedInt(t, "memory", "size"))
	if size == 0 {
		return ""
	}
	return units.HumanSize(float64(size))
}

// strField returns a string field or "" when absent / not a string.
func strField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// boolField returns a bool field or false when absent / not a bool.
func boolField(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// nestedInt walks a chain of map keys and returns the leaf as an int, or 0.
func nestedInt(m map[string]any, keys ...string) int {
	var cur any = m
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return 0
		}
		cur = obj[k]
	}
	switch v := cur.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}
