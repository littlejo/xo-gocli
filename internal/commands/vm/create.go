package vm

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagPool        = "pool"
	flagTemplate    = "template"
	flagMemory      = "memory"
	flagSSHKey      = "ssh-key"
	flagCloudConfig = "cloud-config"
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
		sshKey      string
		cloudConfig string
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

--ssh-key <file> injects the public SSH key from the file into the guest
with cloud-init (it must be a template that supports cloud-config, e.g. the
standard Xen Orchestra Linux templates); --cloud-config <file> passes a full
cloud-init user-data file instead (hostname, packages, users, …). The two
flags are mutually exclusive.

Examples:
  xo vm create web-02 --pool <pool-id> --template <template-id>
  xo vm create web-02 --pool <pool-id> --template <template-id> --memory 4G --description "web server"
  xo vm create web-02 --pool <pool-id> --template <template-id> --boot
  xo vm create web-02 --pool <pool-id> --template <template-id> --boot --ssh-key ~/.ssh/id_ed25519.pub`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if poolID == "" {
				return fmt.Errorf("--pool is required (see 'xo pool list')")
			}
			if templateID == "" {
				return fmt.Errorf("--template is required (see 'xo template list')")
			}
			if sshKey != "" && cloudConfig != "" {
				return fmt.Errorf("--ssh-key and --cloud-config are mutually exclusive")
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
			if sshKey != "" || cloudConfig != "" {
				cc, err := cloudConfigFromFlags(sshKey, cloudConfig)
				if err != nil {
					return err
				}
				params.CloudConfig = &cc
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
	flags.StringVar(&sshKey, flagSSHKey, "", "path to a public SSH key to inject into the guest with cloud-init (the template must support cloud-config)")
	flags.StringVar(&cloudConfig, flagCloudConfig, "", "path to a cloud-init user-data file (YAML) passed to the guest; mutually exclusive with --ssh-key")

	return cmd
}

// cloudConfigFromFlags builds the cloud-init user-data passed to create_vm
// (CreateVMParams.CloudConfig): either the file given with --cloud-config, or
// a minimal document that authorizes the public key from --ssh-key.
func cloudConfigFromFlags(sshKey, cloudConfig string) (string, error) {
	if cloudConfig != "" {
		data, err := os.ReadFile(cloudConfig)
		if err != nil {
			return "", fmt.Errorf("cannot read --cloud-config file: %w", err)
		}
		text := string(data)
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("--cloud-config file %q is empty", cloudConfig)
		}
		// Without the magic header, cloud-init may treat the content as a
		// script (or ignore it) and boot the VM with none of the intended
		// configuration applied — a failure only visible inside the guest.
		// Catch it here, at the CLI.
		if !strings.HasPrefix(strings.TrimSpace(text), "#cloud-config") {
			return "", fmt.Errorf("--cloud-config file %q does not start with the '#cloud-config' header; cloud-init will not apply it without it", cloudConfig)
		}
		return text, nil
	}

	data, err := os.ReadFile(sshKey)
	if err != nil {
		return "", fmt.Errorf("cannot read --ssh-key file: %w", err)
	}
	key := strings.TrimSpace(string(data))
	// A public key line starts with an algorithm tag (ssh-ed25519,
	// ecdsa-sha2-nistp256, ssh-rsa, …) followed by base64 and an optional
	// comment. A private key ("-----BEGIN …") or anything else would be
	// a mistake (and would not work in the guest anyway).
	if !strings.HasPrefix(key, "ssh-") && !strings.HasPrefix(key, "ecdsa-") {
		return "", fmt.Errorf("%q does not look like a public SSH key (expected a line starting with ssh-ed25519, ssh-rsa, ecdsa-…, e.g. ~/.ssh/id_ed25519.pub)", sshKey)
	}
	// Multi-line content (e.g. a whole authorized_keys file) would leave
	// stray lines inside the generated document: the API call would still
	// succeed, but the guest would fail to parse the user-data and boot
	// without the key.
	if strings.ContainsAny(key, "\r\n") {
		return "", fmt.Errorf("%q contains multiple lines: --ssh-key expects a single public key line (for authorized_keys-style files, use --cloud-config)", sshKey)
	}
	// The key (including its optional comment) is embedded in a YAML
	// document; quote it with yaml.Marshal so a comment containing YAML
	// special characters (e.g. "user: host") cannot break the document.
	quoted, err := yaml.Marshal(key)
	if err != nil {
		return "", fmt.Errorf("cannot encode --ssh-key content as YAML: %w", err)
	}
	return "#cloud-config\nssh_authorized_keys:\n  - " + strings.TrimSpace(string(quoted)) + "\n", nil
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
		// Show the derived state (in-flight operations take precedence over
		// the lagging raw power_state, as in 'xo vm list' / 'xo vm get'), and
		// only suggest starting the VM when it is actually halted. A VM
		// created with --boot — or from an auto-boot template, without the
		// flag — is already running or starting, so the hint would be noise;
		// if the boot has not (yet) taken, the VM is halted and the hint is
		// the right next step.
		state := vmState(vm)
		extra := ""
		if state == "Halted" {
			extra = fmt.Sprintf("\nStart it with: xo vm start %s\n", vm.ID.String())
		}
		_, err := fmt.Fprintf(w, "VM %q created:\n  id:     %s\n  state:  %s\n  memory: %s\n  cpus:   %d\n%s",
			vm.NameLabel, vm.ID.String(), state, memoryText(vm), vm.CPUs.Number, extra)
		return err
	}
}
