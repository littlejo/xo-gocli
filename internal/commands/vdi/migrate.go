package vdi

import (
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

const flagMigrateSR = "sr"

func newMigrateCommand() *cobra.Command {
	var srID string

	cmd := &cobra.Command{
		Use:   "migrate <id> --sr <sr-id>",
		Short: "Migrate a virtual disk (VDI) to another storage repository",
		Long: `Migrate a virtual disk (VDI) to another storage repository (SR).

The VDI is referenced by its UUID, as returned by 'xo vdi list'. The target
SR must belong to the same pool as the VDI (see 'xo sr list').

The operation is asynchronous: Xen Orchestra returns a task and the command
prints it. Track it with 'xo task get <task-id>' or
'xo task wait <task-id>'.

Note: after the migration completes, the VDI has a NEW id. Look it up again
with 'xo vdi list' (filter by name) or 'xo vm vdis <vm-id>' to get the new
reference.`,
		Example: `  xo vdi migrate 11111111-1111-4111-8111-111111111111 --sr aaaaaaaa-bbbb-cccc-dddd-000000000002
  xo vdi migrate <id> --sr <sr-id> --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if srID == "" {
				return fmt.Errorf("--sr is required (see 'xo sr list')")
			}
			sr, err := uuid.FromString(srID)
			if err != nil {
				return fmt.Errorf("invalid --sr id %q (expected a UUID)", srID)
			}
			id, err := parseID(args[0])
			if err != nil {
				return err
			}

			xo, cfg, err := cli.LoadClient(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// Existence checks through the typed services, so a missing VDI or
			// SR fails with the usual "not found" error before the migration.
			vdi, err := xo.VDI().Get(ctx, id)
			if err != nil {
				return notFound(args[0], err, cfg.Insecure)
			}
			if _, err := xo.SR().Get(ctx, sr); err != nil {
				return cli.NotFound("SR", "resolve", srID, err, cfg.Insecure)
			}
			name := vdi.NameLabel
			if name == "" {
				name = id.String()
			}

			taskID, err := xo.VDI().Migrate(ctx, id, sr)
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot migrate VDI %q: %v", name, err), cfg.Insecure)
			}

			return renderMigrated(cmd, name, taskID)
		},
	}

	cmd.Flags().StringVar(&srID, flagMigrateSR, "", "target storage repository UUID (required; see 'xo sr list')")
	return cmd
}

// renderMigrated prints the outcome of an async VDI migration: a friendly line
// for the human formats and a small machine document for json/yaml.
func renderMigrated(cmd *cobra.Command, name, taskID string) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"action": "migrate", "vdi": name, "task_id": taskID})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "Requested migration of VDI %q (task %s)\n", name, taskID)
		return err
	}
}
