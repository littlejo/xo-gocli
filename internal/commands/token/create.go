package token

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagDescription = "description"
	flagClientID    = "client-id"
	flagExpiresIn   = "expires-in"
)

func newCreateCommand() *cobra.Command {
	var (
		description string
		clientID    string
		expiresIn   string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an authentication token",
		Long: `Create an authentication token for the current user.

The created token is printed once in full (its id is the secret). Save it
now: it can be listed later but only masked, unless you kept the value.

If --client-id is given and a non-expired token already exists for that
client id, the existing token is updated and returned instead of creating a
new one.`,
		Example: `  xo token create
  xo token create --description "ci pipeline" --expires-in "30 days"
  xo token create --client-id my-cli --expires-in "1 hour"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			body := map[string]any{}
			if description != "" {
				body["description"] = description
			}
			if clientID != "" {
				body["client"] = map[string]any{"id": clientID}
			}
			if expiresIn != "" {
				body["expiresIn"] = expiresIn
			}
			encoded, err := json.Marshal(body)
			if err != nil {
				return err
			}

			respBody, err := doTokensRequest(cmd.Context(), httpClient, "POST", tokensEndpoint, encoded, nil)
			if err != nil {
				return cli.APIError(err, cfg.Insecure, "cannot create token")
			}

			var resp struct {
				Token map[string]any `json:"token"`
			}
			if err := json.Unmarshal(respBody, &resp); err != nil || resp.Token == nil {
				return fmt.Errorf("cannot create token: unexpected response: %s", truncate(string(respBody), 200))
			}

			return renderCreatedToken(cmd, format, resp.Token)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&description, flagDescription, "", "description for the token")
	flags.StringVar(&clientID, flagClientID, "", "client identifier (reuses the existing token for this client if any)")
	flags.StringVar(&expiresIn, flagExpiresIn, "", "token lifetime, e.g. \"1 hour\", \"30 days\" (default set by the server)")

	return cmd
}

// renderCreatedToken prints the newly created token. Unlike list/get, the
// secret (the token id) is printed in full: this is the moment the user can
// copy it (for example into 'xo configure').
func renderCreatedToken(cmd *cobra.Command, format output.Format, t map[string]any) error {
	w := cmd.OutOrStdout()
	switch format {
	case output.FormatJSON, output.FormatYAML:
		normalized, err := output.Normalize(t)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, normalized, nil)
	default:
		_, err := fmt.Fprintf(w, "Token created:\n  id:          %s\n  description: %s\n  created:     %s\n  expires:     %s\n\nSave this token now (for example: xo configure --token <id>); it is not shown again in full by 'xo token list'.\n",
			strField(t, "id"), strField(t, "description"), timeText(t["created_at"]), timeText(t["expiration"]))
		return err
	}
}

// truncate shortens s to n characters, appending an ellipsis when cut.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
