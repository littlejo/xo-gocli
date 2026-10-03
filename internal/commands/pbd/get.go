package pbd

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

func newGetCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a physical block device (PBD)",
		Long: `Get a physical block device (PBD) from Xen Orchestra, as a detailed,
human readable view.

A PBD is the connection between a host and a storage repository (SR); it
is what "plugs" an SR into a host. The PBD is referenced by its UUID, as
returned by 'xo pbd list'.

The human (default) view is a detail sheet that resolves the relationships
by name (the host, the SR and the pool) and shows the full device_config,
not just the block device (for example the server/serverpath of an NFS SR).
If a reference cannot be resolved, the raw id is shown instead and the
command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo pbd get <id> --output json' is unchanged.

Examples:
  xo pbd get 550e8400-e29b-41d4-a716-446655440001
  xo pbd get <id> --output json
  xo pbd get <id> --query 'device_config'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if err := output.ValidateQuery(query); err != nil {
				return err
			}
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}

			xo, cfg, err := newClient(cmd)
			if err != nil {
				return err
			}

			pbd, err := xo.PBD().Get(cmd.Context(), id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}

			// The structured formats (json/yaml/text) and --query are served
			// straight from the raw object, exactly as before; only the default
			// human view is the new detail sheet, which resolves relationships.
			if format != output.FormatTable || query != "" {
				return renderPBDRaw(cmd.OutOrStdout(), format, pbd, query)
			}

			resolver, err := cli.NewResolver(cmd, cfg)
			if err != nil {
				return err
			}
			return renderPBDDetail(cmd.OutOrStdout(), cmd.Context(), pbd, resolver)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'device_config'")
	return cmd
}

// parseID converts a positional PBD identifier into a UUID.
func parseID(id string) (uuid.UUID, error) {
	u, err := uuid.FromString(id)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid PBD id %q (expected a UUID)", id)
	}
	return u, nil
}

// newClient loads the selected profile and builds an authenticated SDK v2
// client. Commands must pass their cobra context to the SDK operations so that
// cancellation (Ctrl+C) reaches the HTTP layer.
func newClient(cmd *cobra.Command) (library.Library, *config.ClientConfig, error) {
	cfg, err := config.Load(cli.ProfileName(cmd))
	if err != nil {
		return nil, nil, err
	}
	xo, err := cli.NewClient(cmd, cfg)
	if err != nil {
		return nil, nil, err
	}
	return xo, cfg, nil
}

// notFound delegates to cli.NotFound, which reports a 404 concisely and keeps
// the raw API error as debug detail.
func notFound(id string, err error, insecure bool) error {
	return cli.NotFound("PBD", "get", id, err, insecure)
}

// renderPBDRaw renders the PBD in the structured formats (json/yaml/text) or
// as a --query projection. It emits the raw object so machine readable output
// and the query pipeline are unchanged.
func renderPBDRaw(w io.Writer, format output.Format, pbd *payloads.PBD, query string) error {
	if query != "" {
		queryResult, err := output.Query(query, pbd)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, pbd, queryResult)
	}

	normalized, err := output.Normalize(pbd)
	if err != nil {
		return err
	}
	return output.Render(w, format, output.Table{}, normalized, nil)
}

// renderPBDDetail renders the single PBD as a human readable detail sheet.
// The three relationships (host, SR, pool) are resolved to names by the
// resolver at a constant cost. The full device_config is shown so that, for
// example, an NFS SR reveals its server and serverpath. A reference that
// cannot be resolved falls back to its raw id, so the view is always complete.
func renderPBDDetail(w io.Writer, ctx context.Context, pbd *payloads.PBD, r *resolve.Client) error {
	lines := make([]string, 0, 8)

	// Header: type and (attachment).
	header := "PBD  (attached=" + boolText(pbd.Attached) + ")"
	if dev := deviceOf(pbd); dev != "-" {
		header = "PBD " + dev + "  (attached=" + boolText(pbd.Attached) + ")"
	}
	lines = append(lines, header)

	// Relationships (resolved by name; the resolver falls back to the raw id
	// when a reference cannot be resolved, so the view is always complete).
	if name, err := r.HostName(ctx, pbd.Host); err == nil {
		lines = append(lines, output.DetailField("Host", name))
	} else {
		lines = append(lines, output.DetailField("Host", pbd.Host.String()))
	}
	if name, err := r.SRName(ctx, pbd.SR); err == nil {
		lines = append(lines, output.DetailField("SR", name))
	} else {
		lines = append(lines, output.DetailField("SR", pbd.SR.String()))
	}
	if name, err := r.PoolName(ctx, pbd.Pool); err == nil {
		lines = append(lines, output.DetailField("Pool", name))
	} else {
		lines = append(lines, output.DetailField("Pool", pbd.Pool.String()))
	}

	// Storage configuration (the full device_config, not just the device).
	if config := deviceConfigText(pbd.DeviceConfig); config != "" {
		lines = append(lines, output.DetailField("Config", config))
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// deviceConfigText renders the whole device_config as "k=v, k=v" in a stable
// (sorted) order, or "" when the PBD carries no device configuration.
func deviceConfigText(cfg map[string]string) string {
	if len(cfg) == 0 {
		return ""
	}
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+cfg[k])
	}
	return strings.Join(parts, ", ")
}
