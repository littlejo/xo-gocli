package vm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagPool     = "pool"
	flagTemplate = "template"
	flagMemory   = "memory"
)

// A UUID rendered as a string is always 36 chars (8-4-4-4-12), so the
// composite vm-templates id printed by 'xo template list' is exactly
// 36 + 1 + 36 = 73 chars: <poolId>-<templateUuid>.
const (
	uuidStringLen  = 36
	compositeIDLen = uuidStringLen + 1 + uuidStringLen
)

func newCreateCommand() *cobra.Command {
	var (
		poolID      string
		templateID  string
		description string
		memory      string
		boot        bool
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a virtual machine in a pool",
		Long: `Create a virtual machine in a pool from a template.

The pool is referenced by its UUID, as returned by 'xo pool list'. The
template accepts the id printed by 'xo template list' (the composite
<poolId>-<templateUuid> id) or the bare template UUID (the 'uuid' field of
'xo template list --output json'); both refer to the same template. The
template must belong to the given pool.

Memory, when given, is expressed in bytes or as a human readable size
(e.g. 2G, 512M). If omitted, the template default is used.

Use --boot to start the VM as soon as it has been created.

Examples:
  xo vm create web-02 --pool <pool-id> --template <template-id>
  xo vm create web-02 --pool <pool-id> --template <template-id> --memory 4G --description "web server"
  xo vm create web-02 --pool <pool-id> --template <template-id> --boot`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if poolID == "" {
				return fmt.Errorf("--pool is required (see 'xo pool list')")
			}
			if templateID == "" {
				return fmt.Errorf("--template is required (see 'xo template list')")
			}
			pool, err := uuid.FromString(poolID)
			if err != nil {
				return fmt.Errorf("invalid --pool id %q (expected a UUID)", poolID)
			}
			// --template accepts either the bare template UUID or the
			// composite <poolId>-<templateUuid> id that 'xo template list'
			// prints. create_vm wants the bare template uuid, so reduce the
			// composite form to it.
			template, err := parseTemplateID(templateID)
			if err != nil {
				return err
			}

			params := &payloads.CreateVMParams{
				NameLabel:       args[0],
				NameDescription: description,
				Template:        template,
			}
			if boot {
				b := true
				params.Boot = &b
			}
			if memory != "" {
				bytes, err := parseMemory(memory)
				if err != nil {
					return err
				}
				m := int(bytes)
				params.Memory = &m
			}

			xo, cfg, err := newClient(cmd)
			if err != nil {
				return err
			}
			vm, err := xo.VM().Create(cmd.Context(), pool, params)
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot create VM %q: %v", args[0], err), cfg.Insecure)
			}

			return renderCreatedVM(cmd, vm)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&poolID, flagPool, "", "pool UUID to create the VM in (see 'xo pool list')")
	flags.StringVar(&templateID, flagTemplate, "", "template id to base the VM on: the bare template UUID or the composite id printed by 'xo template list'")
	flags.StringVar(&description, "description", "", "description for the VM")
	flags.StringVar(&memory, flagMemory, "", "memory size, in bytes or human readable (e.g. 2G, 512M)")
	flags.BoolVar(&boot, "boot", false, "start the VM as soon as it has been created")

	return cmd
}

// parseTemplateID parses --template, accepting either the bare template UUID
// or the composite <poolId>-<templateUuid> id that 'xo template list' prints.
// create_vm wants the bare template uuid, so the composite form is reduced to
// its trailing UUID.
func parseTemplateID(id string) (uuid.UUID, error) {
	fail := func() error {
		return fmt.Errorf("invalid --template id %q (expected a UUID, or the composite poolId-templateUuid id printed by 'xo template list')", id)
	}
	if len(id) == compositeIDLen {
		if _, err := uuid.FromString(id[:uuidStringLen]); err != nil {
			return uuid.Nil, fail()
		}
		template, err := uuid.FromString(id[uuidStringLen+1:])
		if err != nil {
			return uuid.Nil, fail()
		}
		return template, nil
	}
	template, err := uuid.FromString(id)
	if err != nil {
		return uuid.Nil, fail()
	}
	return template, nil
}

// parseMemory turns a size like "2G", "512M" or "2147483648" into bytes.
func parseMemory(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("--memory cannot be empty")
	}

	multiplier := int64(1)
	digits := trimmed
	switch {
	case strings.HasSuffix(trimmed, "GiB") || strings.HasSuffix(trimmed, "G"):
		multiplier = 1 << 30
		digits = strings.TrimSuffix(strings.TrimSuffix(trimmed, "GiB"), "G")
	case strings.HasSuffix(trimmed, "MiB") || strings.HasSuffix(trimmed, "M"):
		multiplier = 1 << 20
		digits = strings.TrimSuffix(strings.TrimSuffix(trimmed, "MiB"), "M")
	case strings.HasSuffix(trimmed, "KiB") || strings.HasSuffix(trimmed, "K"):
		multiplier = 1 << 10
		digits = strings.TrimSuffix(strings.TrimSuffix(trimmed, "KiB"), "K")
	}

	n, err := strconv.ParseInt(strings.TrimSpace(digits), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid --memory %q (expected a whole number of bytes or a human readable size, e.g. 2G, 512M)", value)
	}
	if n <= 0 {
		return 0, fmt.Errorf("--memory must be greater than zero")
	}
	return n * multiplier, nil
}

// renderCreatedVM prints the newly created VM in the requested format.
func renderCreatedVM(cmd *cobra.Command, vm *payloads.VM) error {
	format, err := output.ParseFormat(cli.OutputFormat(cmd))
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		normalized, err := output.Normalize(vm)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, normalized, nil)
	default:
		_, err := fmt.Fprintf(w, "VM %q created:\n  id:     %s\n  state:  %s\n  memory: %s\n  cpus:   %d\n\nStart it with: xo vm start %s\n",
			vm.NameLabel, vm.ID.String(), vm.PowerState, memoryText(vm), vm.CPUs.Number, vm.ID.String())
		return err
	}
}
