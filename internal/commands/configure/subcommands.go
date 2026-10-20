package configure

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

// maskSecret masks a stored credential so that 'list' and 'show' never print
// a full token or password, matching the masking of 'xo token list'. It keeps
// a tail so the user can still tell which secret is stored.
func maskSecret(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	return "****" + value[len(value)-4:]
}

// profileRow renders one profile as a table row. Secrets are masked.
func profileRow(current string, p config.Profile) []string {
	auth := "token" + maskSecret(p.Token)
	if p.Token == "" {
		auth = "password (" + p.Username + ")"
	}
	return []string{
		p.Name,
		markCurrent(current, p.Name),
		p.Endpoint,
		auth,
		boolText(p.Insecure),
	}
}

func markCurrent(current, name string) string {
	if current == name {
		return "yes"
	}
	return ""
}

func boolText(b bool) string {
	if b {
		return "yes"
	}
	return ""
}

// renderProfiles applies the optional --query expression and renders the
// profiles in the requested format.
func renderProfiles(w io.Writer, format output.Format, current string, profiles []config.Profile, query string) error {
	rows := make([][]string, 0, len(profiles))
	for _, p := range profiles {
		rows = append(rows, profileRow(current, p))
	}

	// The structured projection works on plain maps (secrets masked).
	raw := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		m := map[string]any{
			"name":     p.Name,
			"current":  current == p.Name,
			"endpoint": p.Endpoint,
			"insecure": p.Insecure,
		}
		if p.Token != "" {
			m["token"] = maskSecret(p.Token)
		} else if p.Username != "" {
			m["username"] = p.Username
			m["password"] = maskSecret(p.Password)
		}
		raw = append(raw, m)
	}

	if query == "" {
		return output.Render(w, format, output.Table{
			Headers: []string{"NAME", "CURRENT", "ENDPOINT", "AUTH", "INSECURE"},
			Rows:    rows,
		}, raw, nil)
	}

	queryResult, err := output.Query(query, raw)
	if err != nil {
		return err
	}
	return output.Render(w, format, output.Table{}, raw, queryResult)
}

func newListCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configuration profiles",
		Long: `List the stored configuration profiles.

Credentials are masked; 'show' reveals a single profile in full.`,
		Example: `  xo configure list
  xo configure list --output json
  xo configure list --query '[].name'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}
			if err := output.ValidateQuery(query); err != nil {
				return err
			}

			current, profiles, err := config.List()
			if err != nil {
				return err
			}

			return renderProfiles(cmd.OutOrStdout(), format, current, profiles, query)
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "JMESPath expression applied to the result, e.g. '[].name'")
	return cmd
}

func newShowCommand() *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "show [name]",
		Short: "Show a configuration profile",
		Long: `Show a stored configuration profile in full, including the token or
password. Without a name, the selected profile is shown (--profile or
$XOA_PROFILE; the current profile when neither is set).`,
		Example: `  xo configure show
  xo configure show lab
  xo configure show lab --output json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}
			if err := output.ValidateQuery(query); err != nil {
				return err
			}

			var name string
			if len(args) > 0 {
				name = args[0]
			} else if name, err = selectedProfile(cmd); err != nil {
				return err
			}

			profile, err := config.GetProfile(name)
			if err != nil {
				return err
			}

			if query == "" {
				return output.Render(cmd.OutOrStdout(), format, output.Table{
					Headers: []string{"NAME", "ENDPOINT", "AUTH", "INSECURE"},
					Rows: [][]string{{
						profile.Name,
						profile.Endpoint,
						authDescription(*profile),
						boolText(profile.Insecure),
					}},
				}, profile, nil)
			}

			queryResult, err := output.Query(query, profile)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, output.Table{}, profile, queryResult)
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "JMESPath expression applied to the result, e.g. 'endpoint'")
	return cmd
}

func authDescription(p config.Profile) string {
	if p.Token != "" {
		return "token " + p.Token
	}
	return "password (" + p.Username + "): " + p.Password
}

func newRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a configuration profile",
		Long: `Remove a stored configuration profile.

This is a destructive operation and asks for confirmation unless --yes is
given (or $XOA_YES is set). When the removed profile was the current one,
current falls back to the first remaining profile.`,
		Example: `  xo configure remove lab
  xo configure remove lab --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("profile name must not be empty")
			}

			if !cli.SkipConfirm(cmd) {
				ok, err := cli.Confirm(cmd, fmt.Sprintf("Remove profile %q?", name), false)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("aborted")
				}
			}

			_, err := config.Remove(name)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Profile %q removed.\n", name); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().Bool(flagYes, false, "do not ask for confirmation (or set XOA_YES=1)")
	return cmd
}

// flagYes skips the interactive confirmation of 'configure remove'.
const flagYes = "yes"

func newUseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Make a profile the current one",
		Long: `Make a stored profile the current one: the profile every command uses when
neither --profile nor $XOA_PROFILE is set. 'xo configure list' marks it.`,
		Example: `  xo configure use lab
  xo configure list`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("profile name must not be empty")
			}
			path, err := config.SetCurrent(name)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.ErrOrStderr(), "Profile %q is now the current profile (%s)\n", name, path)
			return err
		},
	}
}
