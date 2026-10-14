package token

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

func newGetCommand() *cobra.Command {
	var (
		query    string
		noSecret bool
	)

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get an authentication token",
		Long: `Get an authentication token of the current user.

The token is referenced by its id (the full secret value, as returned by
'xo token create' or 'xo token list --no-secret'). The REST API does not
expose a per-token endpoint, so this is resolved against the token list.

The token id is the secret, so it is masked in the output by default;
pass --no-secret to reveal the full value (use with care).`,
		Example: `  xo token get <id>
  xo token get <id> --output json
  xo token get <id> --query 'description'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if id == "" {
				return fmt.Errorf("invalid token id %q", id)
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

			body, err := doTokensRequest(cmd.Context(), httpClient, "GET", tokensEndpoint, nil, nil)
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot get token %q: %v", id, err), cfg.Insecure)
			}

			var tokens []map[string]any
			if err := json.Unmarshal(body, &tokens); err != nil {
				return fmt.Errorf("cannot get token %q: %v", id, err)
			}

			var found map[string]any
			for _, t := range tokens {
				if strField(t, "id") == id {
					found = t
					break
				}
			}
			if found == nil {
				return fmt.Errorf("token %q not found", id)
			}

			return renderToken(cmd.OutOrStdout(), format, found, query, noSecret)
		},
	}

	cmd.Flags().StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. 'description'")
	cmd.Flags().BoolVar(&noSecret, flagNoSecret, false, "do not mask the token secret in the output")
	return cmd
}

// renderToken renders a single token in the requested format. The human format
// uses a single-row table with the same columns as 'xo token list'; the
// structured formats emit the normalized object (or its --query projection).
// The token secret (its id) is masked unless --no-secret is given.
func renderToken(w io.Writer, format output.Format, t map[string]any, query string, noSecret bool) error {
	if !noSecret {
		masked := make(map[string]any, len(t))
		for k, v := range t {
			masked[k] = v
		}
		if id, ok := masked["id"].(string); ok {
			masked["id"] = maskToken(id)
		}
		t = masked
	}

	if format == output.FormatTable && query == "" {
		table := output.Table{
			Headers: []string{"ID", "DESCRIPTION", "CREATED", "EXPIRES", "CLIENT"},
			Rows:    [][]string{tokenRow(t)},
		}
		return output.Render(w, format, table, t, nil)
	}

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
