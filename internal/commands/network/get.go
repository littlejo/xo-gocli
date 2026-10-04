package network

import (
	"context"
	"fmt"
	"io"
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
		Short: "Get a network",
		Long: `Get a network from Xen Orchestra, as a detailed, human readable view.

The network is referenced by its UUID, as returned by 'xo network list'.

The human (default) view is a detail sheet: the bridge, MTU and type, the pool
it belongs to (shown by name), and the counts of VIFs and PIFs. The pool
relationship is resolved by a single lookup; if it cannot be resolved, the raw
id is shown instead and the command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo network get <id> --output json' is unchanged.

Examples:
  xo network get 11111111-1111-4111-8111-111111111111
  xo network get <id> --output json
  xo network get <id> --query 'name_label'`,
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

			network, err := xo.Network().Get(cmd.Context(), id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}

			// The structured formats (json/yaml/text) and --query are served
			// straight from the raw object, exactly as before; only the default
			// human view is the new detail sheet, which resolves the pool.
			if format != output.FormatTable || query != "" {
				return renderNetworkRaw(cmd.OutOrStdout(), format, network, query)
			}

			resolver, err := cli.NewResolver(cmd, cfg)
			if err != nil {
				return err
			}
			return renderNetworkDetail(cmd.OutOrStdout(), cmd.Context(), network, resolver)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'name_label'")
	return cmd
}

// parseID converts a positional network identifier into a UUID.
func parseID(id string) (uuid.UUID, error) {
	u, err := uuid.FromString(id)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid network id %q (expected a UUID)", id)
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
	return cli.NotFound("network", "get", id, err, insecure)
}

// renderNetworkRaw renders the network in the structured formats (json/yaml/text) or
// as a --query projection. It emits the raw object so machine readable output
// and the query pipeline are unchanged.
func renderNetworkRaw(w io.Writer, format output.Format, n *payloads.Network, query string) error {
	if query != "" {
		queryResult, err := output.Query(query, n)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, n, queryResult)
	}

	normalized, err := output.Normalize(n)
	if err != nil {
		return err
	}
	return output.Render(w, format, output.Table{}, normalized, nil)
}

// renderNetworkDetail renders the single network as a human readable detail
// sheet. The pool is resolved by name (a single constant lookup); a reference
// that cannot be resolved falls back to its raw id, so the view is always
// complete.
func renderNetworkDetail(w io.Writer, ctx context.Context, n *payloads.Network, r *resolve.Client) error {
	lines := make([]string, 0, 12)

	// Header: name and (type).
	header := "Network " + n.NameLabel
	if n.Type != "" {
		header += "  (" + string(n.Type) + ")"
	}
	lines = append(lines, header)

	// Identity
	if n.NameDescription != "" {
		lines = append(lines, output.DetailField("Description", n.NameDescription))
	}
	if len(n.Tags) > 0 {
		lines = append(lines, output.DetailField("Tags", strings.Join(n.Tags, ", ")))
	}

	// Location (resolved by name; the resolver falls back to the raw id when
	// the pool cannot be resolved, so the view is always complete).
	if !n.Pool.IsNil() {
		name, _ := r.PoolName(ctx, n.Pool)
		lines = append(lines, output.DetailField("Pool", output.OrDash(name)))
	}

	// Configuration
	lines = append(lines, output.DetailField("Bridge", output.OrDash(n.Bridge)))
	lines = append(lines, output.DetailField("MTU", fmt.Sprintf("%d", n.MTU)))
	if n.IsBonded {
		lines = append(lines, output.DetailField("Bonded", "yes"))
	}
	if n.DefaultIsLocked {
		lines = append(lines, output.DetailField("Default", "locked"))
	}
	if n.NBD != nil && *n.NBD {
		lines = append(lines, output.DetailField("NBD", "yes"))
	}

	// Attachments (counts only: PIFs have no typed SDK service yet, so listing
	// the hosts is not possible without a second REST surface).
	lines = append(lines, output.DetailField("VIFs", fmt.Sprintf("%d", len(n.VIFs))))
	lines = append(lines, output.DetailField("PIFs", fmt.Sprintf("%d", len(n.PIFs))))

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}
