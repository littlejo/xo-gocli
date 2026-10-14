// Package configure implements 'xo configure', the command that stores
// connection profiles.
package configure

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
)

// NewCommand builds the 'xo configure' command.
func NewCommand() *cobra.Command {
	var (
		endpoint string
		token    string
		username string
		password string
		insecure bool
	)

	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Manage configuration profiles",
		Long: `Initialize or update a Xen Orchestra connection profile, or inspect and
manage the stored profiles.

Without a sub-command, 'xo configure' stores (or updates) the profile
selected with --profile: values can be provided with flags or interactively
(when stdin is a terminal). Environment variables take precedence over the
stored profile at run time.

Sub-commands:
  xo configure list           list the stored profiles
  xo configure show [name]    show one profile (the selected one by default)
  xo configure remove <name>  remove a stored profile`,
		Example: `  xo configure
  xo configure --profile lab --endpoint https://xo.example.com --token <token>
  xo configure --profile lab --username admin --password <secret>
  xo configure list
  xo configure remove lab`,
		// No positional argument: without this, a typo of a sub-command
		// ('xo configure lsit') would run the store path and silently
		// overwrite the selected profile with the XOA_* environment values.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			in := cmd.InOrStdin()
			out := cmd.ErrOrStderr()
			interactive := isTerminal(in)

			if endpoint == "" {
				endpoint = os.Getenv(config.EnvEndpoint)
			}
			if endpoint == "" {
				var err error
				if endpoint, err = prompt(in, out, "endpoint", interactive); err != nil {
					return err
				}
			}
			if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
				return fmt.Errorf("invalid endpoint %q: it must start with http:// or https://", endpoint)
			}

			if token == "" {
				token = os.Getenv(config.EnvToken)
			}
			if token == "" {
				if username == "" {
					username = os.Getenv(config.EnvUsername)
				}
				if username == "" {
					if interactive {
						var err error
						if username, err = prompt(in, out, "username", true); err != nil {
							return err
						}
					} else {
						return fmt.Errorf("--token or --username/--password is required")
					}
				}
				if password == "" {
					password = os.Getenv(config.EnvPassword)
				}
				if password == "" {
					if interactive {
						var err error
						if password, err = promptSecret(in, out, "password"); err != nil {
							return err
						}
					} else {
						return fmt.Errorf("--password is required when --username is set")
					}
				}
			}

			profile := config.Profile{
				Name:     normalizeProfileName(cli.ProfileName(cmd)),
				Endpoint: endpoint,
				Token:    token,
				Username: username,
				Password: password,
				Insecure: insecure,
			}

			path, err := config.Upsert(profile, true)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "Profile %q saved to %s\n", profile.Name, path); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&endpoint, "endpoint", "", "Xen Orchestra endpoint, e.g. https://xo.example.com")
	flags.StringVar(&token, "token", "", "authentication token (or use --username/--password)")
	flags.StringVar(&username, "username", "", "username (alternative to --token)")
	flags.StringVar(&password, "password", "", "password (alternative to --token)")
	flags.BoolVarP(&insecure, "insecure", "k", false, "skip TLS certificate verification")

	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newShowCommand())
	cmd.AddCommand(newRemoveCommand())

	return cmd
}

func normalizeProfileName(name string) string {
	if strings.TrimSpace(name) == "" {
		return config.DefaultProfile
	}
	return strings.TrimSpace(name)
}

func isTerminal(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func prompt(in io.Reader, out io.Writer, label string, interactive bool) (string, error) {
	if !interactive {
		return "", fmt.Errorf("--%s is required (or set the corresponding XOA_ environment variable)", label)
	}
	if _, err := fmt.Fprintf(out, "%s: ", label); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return "", fmt.Errorf("cannot read %s: %w", label, err)
	}
	return strings.TrimSpace(line), nil
}

func promptSecret(in io.Reader, out io.Writer, label string) (string, error) {
	file, ok := in.(*os.File)
	if !ok {
		return "", fmt.Errorf("%s cannot be read in this context, use --%s", label, label)
	}
	if _, err := fmt.Fprintf(out, "%s: ", label); err != nil {
		return "", err
	}
	secret, err := term.ReadPassword(int(file.Fd()))
	if _, ferr := fmt.Fprintln(out); ferr != nil && err == nil {
		err = ferr
	}
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", label, err)
	}
	return string(secret), nil
}
