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

const (
	flagQuery    = "query"
	flagLimit    = "limit"
	flagNoSecret = "no-secret"
)

func newListCommand() *cobra.Command {
	var (
		query    string
		limit    int
		noSecret bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List authentication tokens",
		Long: `List the authentication tokens of the current user.

Tokens are what the CLI itself uses to authenticate (see 'xo configure').
The token id is the secret, so it is masked in the output by default;
pass --no-secret to reveal the full values (use with care).

Examples:
  xo token list
  xo token list --output json
  xo token list --query '[].description'
  xo token list --no-secret --query '[].id'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}
			if limit < 0 {
				return fmt.Errorf("--limit must be greater than or equal to 0")
			}
			if err := output.ValidateQuery(query); err != nil {
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

			params := map[string]any{}
			if limit > 0 {
				params["limit"] = limit
			}

			body, err := doTokensRequest(cmd.Context(), httpClient, "GET", tokensEndpoint, nil, params)
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("cannot list tokens: %v", err), cfg.Insecure)
			}

			var tokens []map[string]any
			if err := json.Unmarshal(body, &tokens); err != nil {
				return fmt.Errorf("cannot list tokens: %v", err)
			}

			return renderTokens(cmd.OutOrStdout(), format, tokens, query, noSecret)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. '[].description'")
	flags.IntVar(&limit, flagLimit, 0, "maximum number of tokens to return (0 for no limit)")
	flags.BoolVar(&noSecret, flagNoSecret, false, "do not mask the token secret in the output")

	return cmd
}

// renderTokens applies the optional --query expression and renders the result
// in the requested format. The token secret (its id) is masked unless
// --no-secret is given.
func renderTokens(w io.Writer, format output.Format, tokens []map[string]any, query string, noSecret bool) error {
	if !noSecret {
		tokens = maskTokens(tokens)
	}

	queryResult, err := output.Query(query, tokens)
	if err != nil {
		return err
	}

	table := output.Table{
		Headers: []string{"ID", "DESCRIPTION", "CREATED", "EXPIRES", "CLIENT"},
		Empty:   "tokens",
	}
	for _, t := range tokens {
		table.Rows = append(table.Rows, tokenRow(t))
	}

	// For structured formats without a query, normalize so the output only
	// contains the requested data.
	var raw any = tokens
	if format != output.FormatTable && (queryResult == nil || !queryResult.Present) {
		normalized, err := output.Normalize(tokens)
		if err != nil {
			return err
		}
		raw = normalized
	}

	return output.Render(w, format, table, raw, queryResult)
}
