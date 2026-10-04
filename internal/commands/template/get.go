package template

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gofrs/uuid"
	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

func newGetCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a VM template",
		Long: `Get a VM template from Xen Orchestra, as a detailed, human readable view.

The template is referenced by its id, as returned by 'xo template list'.

The human (default) view is a detail sheet: the memory and CPUs, the pool the
template belongs to (shown by name), and the power state. If the pool cannot
be resolved, the raw id is shown instead and the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo template get <id> --output json' is
unchanged.

Examples:
  xo template get d31e47fd-a70e-d849-883e-c17193472710-6959dfe8-534c-4c58-8a8c-3c3792293543
  xo template get <id> --output json
  xo template get <id> --query 'name_label'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if id == "" || strings.Contains(id, "/") {
				return fmt.Errorf("invalid template id %q", id)
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

			var template map[string]any
			endpoint := templatesEndpoint + "/" + id
			if err := client.TypedGet(cmd.Context(), httpClient, endpoint, listParams{Fields: "*"}, &template); err != nil {
				return notFound(id, err, cfg.Insecure)
			}

			// The structured formats (json/yaml/text) and --query are served
			// straight from the raw object, exactly as before; only the default
			// human view is the new detail sheet, which resolves the pool.
			if format != output.FormatTable || query != "" {
				return renderTemplateRaw(cmd.OutOrStdout(), format, template, query)
			}

			resolver, err := cli.NewResolver(cmd, cfg)
			if err != nil {
				return err
			}
			return renderTemplateDetail(cmd.OutOrStdout(), cmd.Context(), template, resolver)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'name_label'")
	return cmd
}

// notFound delegates to cli.NotFound, which reports a 404 concisely and keeps
// the raw API error as debug detail.
func notFound(id string, err error, insecure bool) error {
	return cli.NotFound("template", "get", id, err, insecure)
}

// renderTemplateRaw renders the template in the structured formats (json/yaml/text) or
// as a --query projection. It emits the raw object so machine readable output
// and the query pipeline are unchanged.
func renderTemplateRaw(w io.Writer, format output.Format, t map[string]any, query string) error {
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

// renderTemplateDetail renders the single template as a human readable detail
// sheet. The pool is resolved by name (a single constant lookup); a reference
// that cannot be resolved falls back to its raw id, so the view is always
// complete. The template is served as a raw map (no typed SDK model), so its
// fields are read through the small str/bool/nested helpers.
func renderTemplateDetail(w io.Writer, ctx context.Context, t map[string]any, r *resolve.Client) error {
	lines := make([]string, 0, 10)

	// Header: name and (default).
	name := strField(t, "name_label")
	header := "Template " + name
	if boolField(t, "isDefaultTemplate") {
		header += "  (default)"
	}
	lines = append(lines, header)

	// Identity
	if desc := strField(t, "name_description"); desc != "" {
		lines = append(lines, output.DetailField("Description", desc))
	}
	if tags := strSlicesField(t, "tags"); len(tags) > 0 {
		lines = append(lines, output.DetailField("Tags", strings.Join(tags, ", ")))
	}

	// Location (resolved by name; the resolver falls back to the raw id when
	// the pool cannot be resolved, so the view is always complete).
	if pool := strField(t, "$pool"); pool != "" {
		if id, err := uuid.FromString(pool); err == nil {
			if pname, err := r.PoolName(ctx, id); err == nil {
				lines = append(lines, output.DetailField("Pool", output.OrDash(pname)))
			} else {
				lines = append(lines, output.DetailField("Pool", pool))
			}
		} else {
			lines = append(lines, output.DetailField("Pool", output.OrDash(pool)))
		}
	}

	// Resources
	if s := memoryText(t); s != "" {
		lines = append(lines, output.DetailField("Memory", s))
	}
	if cpus := templateCpuText(t); cpus != "" {
		lines = append(lines, output.DetailField("CPUs", cpus))
	}
	if ps := strField(t, "power_state"); ps != "" {
		lines = append(lines, output.DetailField("Power state", ps))
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// strSlicesField returns a []string field or an empty slice when absent or of
// the wrong type.
func strSlicesField(m map[string]any, key string) []string {
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// templateCpuText renders "n" or "n (max m)" from the template's CPUs object.
func templateCpuText(t map[string]any) string {
	number := nestedInt(t, "CPUs", "number")
	max := nestedInt(t, "CPUs", "max")
	if number <= 0 {
		return ""
	}
	if max > 0 && max != number {
		return fmt.Sprintf("%d (max %d)", number, max)
	}
	return fmt.Sprintf("%d", number)
}
