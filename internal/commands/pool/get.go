package pool

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
	"github.com/littlejo/xo-gocli/internal/output"
	"github.com/littlejo/xo-gocli/internal/resolve"
)

func newGetCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "get <id|name>",
		Short: "Get a pool",
		Long: `Get a pool from Xen Orchestra, as a detailed, human readable view.

The pool is referenced by its UUID, as returned by 'xo pool list'.

The pool can also be given by its exact name (name_label), at the cost of
one more request; a name shared by several pools is an error that lists
their ids.

The human (default) view is a detail sheet that resolves the master host and
the storage repositories (default, suspend and crash-dump) by name, and lists
the pool's hosts by name (resolved in one batch, never one lookup per host).
The platform version, CPU topology, HA and the pool features are shown too.
If a reference cannot be resolved, the raw id is shown instead and the
command still succeeds.

--output json / yaml / text and --query still emit the raw object, so the
machine readable behavior of 'xo pool get <id> --output json' is unchanged.`,
		Example: `  xo pool get 550e8400-e29b-41d4-a716-446655440001
  xo pool get pool-prod
  xo pool get <id> --output json
  xo pool get <id> --query 'name_label'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := output.ValidateQuery(query); err != nil {
				return err
			}
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}

			xo, cfg, err := cli.LoadClient(cmd)
			if err != nil {
				return err
			}
			id, err := lookupID(cmd.Context(), xo, cfg.Insecure, args[0])
			if err != nil {
				return err
			}

			pool, err := xo.Pool().Get(cmd.Context(), id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}

			// The structured formats (json/yaml/text) and --query are served
			// straight from the raw object, exactly as before; only the default
			// human view is the new detail sheet, which resolves relationships.
			if format != output.FormatTable || query != "" {
				return output.RenderRaw(cmd.OutOrStdout(), format, pool, query)
			}

			resolver, err := cli.NewResolver(cmd, cfg)
			if err != nil {
				return err
			}
			return renderPoolDetail(cmd.OutOrStdout(), cmd.Context(), pool, resolver)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'name_label'")
	return cmd
}

// parseID converts a positional pool identifier into a UUID.
func parseID(id string) (uuid.UUID, error) {
	return cli.ParseID("pool", id)
}

// notFound delegates to cli.NotFound, which reports a 404 concisely and keeps
// the raw API error as debug detail.
func notFound(id string, err error, insecure bool) error {
	return cli.NotFound("pool", "get", id, err, insecure)
}

// renderPoolDetail renders the single pool as a human readable detail sheet.
// The master host and the storage repositories are resolved by name (a few
// constant lookups) and the pool's hosts are resolved in one batch (never one
// lookup per host). A reference that cannot be resolved falls back to its raw
// id, so the view is always complete.
func renderPoolDetail(w io.Writer, ctx context.Context, p *payloads.Pool, r *resolve.Client) error {
	lines := make([]string, 0, 16)

	// Header: name and (HA).
	header := "Pool " + p.NameLabel
	if p.HAEnabled {
		header += "  (HA)"
	}
	lines = append(lines, header)

	// Identity
	if p.NameDescription != "" {
		lines = append(lines, output.DetailField("Description", p.NameDescription))
	}
	if len(p.Tags) > 0 {
		lines = append(lines, output.DetailField("Tags", strings.Join(p.Tags, ", ")))
	}

	// Membership (resolved by name; the resolver falls back to the raw id when
	// a reference cannot be resolved, so the view is always complete).
	if !p.Master.IsNil() {
		name, _ := r.HostName(ctx, p.Master)
		lines = append(lines, output.DetailField("Master", output.OrDash(name)))
	}
	if hosts := r.HostsOfPool(ctx, p.ID); len(hosts) > 0 {
		lines = append(lines, output.DetailField("Hosts", output.JoinNames(hosts)))
	}

	// Storage (resolved by name).
	if !p.DefaultSR.IsNil() {
		name, _ := r.SRName(ctx, p.DefaultSR)
		lines = append(lines, output.DetailField("Default SR", output.OrDash(name)))
	}
	if p.SuspendSr != "" {
		if id, err := uuid.FromString(p.SuspendSr); err == nil {
			name, _ := r.SRName(ctx, id)
			lines = append(lines, output.DetailField("Suspend SR", output.OrDash(name)))
		}
	}
	if p.CrashDumpSr != "" {
		if id, err := uuid.FromString(p.CrashDumpSr); err == nil {
			name, _ := r.SRName(ctx, id)
			lines = append(lines, output.DetailField("Crash dump", output.OrDash(name)))
		}
	}

	// Platform
	if p.PlatformVersion != "" {
		lines = append(lines, output.DetailField("Platform", p.PlatformVersion))
	}
	if p.CPUs.Cores > 0 || p.CPUs.Sockets > 0 {
		lines = append(lines, output.DetailField("CPUs", fmt.Sprintf("%d cores, %d sockets", p.CPUs.Cores, p.CPUs.Sockets)))
	}
	if flags := poolFeatures(p); flags != "" {
		lines = append(lines, output.DetailField("Features", flags))
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// poolFeatures renders the non-default pool features as a compact, space
// separated list of human readable tags; empty when everything is at its
// default.
func poolFeatures(p *payloads.Pool) string {
	var f []string
	if p.AutoPoweron {
		f = append(f, "auto-poweron")
	}
	if p.MigrationCompression {
		f = append(f, "migration-compression")
	}
	if p.ZSTDSupported {
		f = append(f, "zstd")
	}
	if p.VTPMSupported {
		f = append(f, "vtpm")
	}
	if len(p.HASRs) > 0 {
		f = append(f, fmt.Sprintf("%d HA SRs", len(p.HASRs)))
	}
	return strings.Join(f, " ")
}

// lookupID resolves the pool a user referenced by UUID or by exact name
// (see cli.LookupID).
func lookupID(ctx context.Context, xo library.Library, insecure bool, ref string) (uuid.UUID, error) {
	return cli.LookupID(ctx, "pool", ref, insecure,
		func(ctx context.Context, filter string) ([]*payloads.Pool, error) {
			return xo.Pool().GetAll(ctx, 0, filter)
		},
		func(o *payloads.Pool) uuid.UUID { return o.ID },
		func(o *payloads.Pool) string { return o.NameLabel })
}
