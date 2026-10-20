package vdi

import (
	"fmt"
	"strings"

	"github.com/docker/go-units"
	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagSR     = "sr"
	flagSize   = "size"
	flagTags   = "tags"
	flagShared = "shared"
)

func newCreateCommand() *cobra.Command {
	var (
		srID        string
		size        string
		description string
		tags        string
		shared      bool
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a virtual disk (VDI) on a storage repository",
		Long: `Create a virtual disk (VDI) on a storage repository (SR).

The SR is referenced by its UUID, as returned by 'xo sr list'. The size,
when given, is expressed in bytes or as a human readable size (e.g. 2G,
512M).

Creating a VDI only allocates the disk on the SR; it is not attached to any
VM yet. Attach it with 'xo vbd create --vm <vm-id> --vdi <id>' (see
docs/usecases.md, 'Add a disk to a VM').`,
		Example: `  xo vdi create data-01 --sr <sr-id> --size 10G
  xo vdi create data-01 --sr <sr-id> --size 10G --description "data disk"
  xo vdi create shared-disk --sr <sr-id> --size 1G --shared --tags common,scratch`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if srID == "" {
				return fmt.Errorf("--sr is required (see 'xo sr list')")
			}
			sr, err := uuid.FromString(srID)
			if err != nil {
				return fmt.Errorf("invalid --sr id %q (expected a UUID)", srID)
			}
			var sizeBytes int64
			if size != "" {
				sizeBytes, err = parseSize(size)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("--size is required (e.g. 10G, 512M, or a number of bytes)")
			}
			tagList, err := parseTags(tags)
			if err != nil {
				return err
			}

			params := payloads.VDICreateParams{
				SRId:            sr,
				VirtualSize:     sizeBytes,
				NameLabel:       args[0],
				NameDescription: description,
				Tags:            tagList,
			}
			if shared {
				b := true
				params.Sharable = &b
			}

			xo, cfg, err := cli.LoadClient(cmd)
			if err != nil {
				return err
			}
			vdiID, err := xo.VDI().Create(cmd.Context(), params)
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot create VDI %q: %v", args[0], err), cfg.Insecure)
			}

			return renderCreated(cmd, vdiID)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&srID, flagSR, "", "SR UUID to create the VDI on (required, see 'xo sr list')")
	flags.StringVar(&size, flagSize, "", "virtual size, in bytes or human readable (e.g. 2G, 512M) (required)")
	flags.StringVar(&description, "description", "", "description for the VDI")
	flags.StringVar(&tags, flagTags, "", "comma-separated tags for the VDI")
	flags.BoolVar(&shared, flagShared, false, "allow the VDI to be attached to several VMs at once")

	return cmd
}

// parseSize turns a size like "2G", "512M" or "2147483648" into bytes.
func parseSize(value string) (int64, error) {
	bytes, err := units.RAMInBytes(value)
	if err != nil || bytes <= 0 {
		return 0, fmt.Errorf("invalid --size %q (expected a whole number of bytes or a human readable size, e.g. 2G, 512M)", value)
	}
	return bytes, nil
}

// parseTags splits a comma-separated tag list into a clean []string, dropping
// empty entries. It returns an error if the input is non-empty but yields no
// usable tags, so that e.g. --tags "," is rejected.
func parseTags(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var tags []string
	for _, part := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("invalid --tags %q (no tags after splitting)", value)
	}
	return tags, nil
}

// renderCreated prints the outcome of a VDI create: the id for every format,
// plus a hint toward the next step (attaching the disk to a VM).
func renderCreated(cmd *cobra.Command, vdiID uuid.UUID) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		raw, err := output.Normalize(map[string]any{"action": "create", "vdi": vdiID.String()})
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, nil)
	default:
		_, err := fmt.Fprintf(w, "VDI created (id %s)\n\nAttach it to a VM with: xo vbd create --vm <vm-id> --vdi %s\n",
			vdiID, vdiID)
		return err
	}
}
