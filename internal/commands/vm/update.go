package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagDescription = "description"
)

// vmEndpointPrefix is the REST resource holding VMs. The SDK v2 exposes a
// typed service for it, but the Update operation is not implemented yet in the
// SDK (VM().Update returns "not yet implemented"). The REST API does support a
// partial update (PATCH /vms/{id}), so this command reaches it through the
// SDK's own REST client (cli.NewHTTPClient) — the same single API boundary, not
// a second HTTP client. The missing typed operation should be contributed to
// the SDK.
const vmEndpointPrefix = "vms"

// authCookieName is the cookie the SDK v2 client uses to carry the token.
const authCookieName = "authenticationToken"

func newUpdateCommand() *cobra.Command {
	var (
		name        string
		description string
		tags        string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update the name, description or tags of a virtual machine",
		Long: `Update the name_label, name_description and/or tags of a virtual machine.

At least one of --name, --description or --tags is required. Only the fields
that are given are changed; the others are left untouched.

--tags REPLACES the full tag list of the VM (it is not a merge). Give the
complete, comma-separated list you want the VM to end up with. To add or
remove a single tag without touching the others, use 'xo vm tag add/remove'.

The VM is referenced by its UUID, as returned by 'xo vm list'.

Note: this operation is not exposed by the SDK v2 typed service yet, so it is
performed through the SDK's own REST client against PATCH /vms/<id>.`,
		Example: `  xo vm update 550e8400-e29b-41d4-a716-446655440001 --name web-01
  xo vm update <id> --description "primary web server"
  xo vm update <id> --tags production,web
  xo vm update <id> --name web-01 --description "primary web server" --tags prod`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tagList, err := parseTags(tags)
			if err != nil {
				return err
			}
			if name == "" && description == "" && tags == "" {
				return fmt.Errorf("nothing to update: provide --name, --description and/or --tags")
			}
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}

			id, err := parseID(args[0])
			if err != nil {
				return err
			}

			cfg, err := config.Load(cli.ProfileName(cmd))
			if err != nil {
				return err
			}

			body := map[string]any{}
			if name != "" {
				body["nameLabel"] = name
			}
			if description != "" {
				body["nameDescription"] = description
			}
			if tags != "" {
				body["tags"] = tagList
			}
			encoded, err := json.Marshal(body)
			if err != nil {
				return err
			}

			httpClient, err := cli.NewHTTPClient(cmd, cfg)
			if err != nil {
				return err
			}

			if err := patchVM(cmd.Context(), httpClient, id.String(), encoded); err != nil {
				return cli.APIError(err, cfg.Insecure, "cannot update VM %q", args[0])
			}

			// Re-fetch through the typed SDK service so the output reflects the
			// updated object and uses the SDK's response types.
			xo, err := cli.NewClient(cmd, cfg)
			if err != nil {
				return err
			}
			vm, err := xo.VM().GetByID(cmd.Context(), id)
			if err != nil {
				return cli.APIError(err, cfg.Insecure, "cannot get updated VM %q", args[0])
			}

			return renderUpdatedVM(cmd, format, vm)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&name, "name", "", "new name_label for the VM")
	flags.StringVar(&description, flagDescription, "", "new name_description for the VM")
	flags.StringVar(&tags, "tags", "", "full comma-separated list of tags to set (replaces the existing list)")

	return cmd
}

// parseTags splits a comma-separated tag list into a clean []string, dropping
// empty entries. It returns an error if the input is non-empty but yields no
// usable tags, so that e.g. --tags "," is rejected rather than clearing tags.
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

// patchVM sends a JSON PATCH to /rest/v0/vms/<id> using the SDK v2 REST client.
// The SDK client's BaseURL is already suffixed with /rest/v0; the endpoint is
// joined to it and the auth cookie is attached.
func patchVM(ctx context.Context, c *client.Client, id string, body []byte) error {
	u := *c.BaseURL
	u.Path = path.Join(c.BaseURL.Path, vmEndpointPrefix, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: string(c.AuthToken)})

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	respBody, err := io.ReadAll(resp.Body)
	if cerr := resp.Body.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error: %s - %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// renderUpdatedVM prints the updated VM in the requested format, using the same
// columns as 'xo vm get'.
func renderUpdatedVM(cmd *cobra.Command, format output.Format, vm *payloads.VM) error {
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		normalized, err := output.Normalize(vm)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, normalized, nil)
	default:
		_, err := fmt.Fprintf(w, "VM %q updated:\n  id:     %s\n  name:   %s\n  desc:   %s\n  tags:   %s\n  state:  %s\n",
			vm.NameLabel, vm.ID.String(), vm.NameLabel, vm.NameDescription, strings.Join(vm.Tags, ", "), vm.PowerState)
		return err
	}
}
