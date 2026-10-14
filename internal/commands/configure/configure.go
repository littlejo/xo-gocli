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

Without a sub-command, 'xo configure' stores the selected profile: the one
named by --profile, else $XOA_PROFILE, else the current profile, else
"default". Values come from the flags, then from the XOA_* environment
variables, then from an interactive prompt (when stdin is a terminal).

Updating an existing profile only changes what you give: the other stored
values (endpoint, credentials, --insecure, the "output" setting) are kept.
Giving a token replaces a stored username and password, and the reverse.
Use --insecure=false to turn TLS verification back on.

The first profile you store becomes the current one. Storing another profile
does not change the current one: select it per command with --profile or
$XOA_PROFILE, or make it current with 'xo configure use <name>'.`,
		Example: `  xo configure
  xo configure --profile lab --endpoint https://xo.example.com --token <token>
  xo configure --profile lab --username admin --password <secret>
  xo configure --profile lab --token <new-token>
  xo configure use lab
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

			name, err := selectedProfile(cmd)
			if err != nil {
				return err
			}
			stored, err := config.Lookup(name)
			if err != nil {
				return err
			}
			_, profiles, err := config.List()
			if err != nil {
				return err
			}
			profile := config.Profile{Name: name}
			if stored != nil {
				profile = *stored
			}

			if endpoint == "" {
				endpoint = os.Getenv(config.EnvEndpoint)
			}
			if endpoint == "" {
				if endpoint, err = prompt(in, out, "endpoint", interactive, profile.Endpoint); err != nil {
					return err
				}
			}
			if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
				return fmt.Errorf("invalid endpoint %q: it must start with http:// or https://", endpoint)
			}
			profile.Endpoint = endpoint

			if token == "" {
				token = os.Getenv(config.EnvToken)
			}
			if username == "" {
				username = os.Getenv(config.EnvUsername)
			}
			if password == "" {
				password = os.Getenv(config.EnvPassword)
			}
			switch {
			case token != "":
				// A token replaces stored password credentials.
				profile.Token, profile.Username, profile.Password = token, "", ""
			case username != "" || password != "":
				if username == "" {
					if username, err = prompt(in, out, "username", interactive, profile.Username); err != nil {
						return err
					}
				}
				if password == "" {
					if !interactive {
						return fmt.Errorf("--password is required when --username is set")
					}
					if password, err = promptSecret(in, out, "password"); err != nil {
						return err
					}
				}
				profile.Token, profile.Username, profile.Password = "", username, password
			case profile.Token != "" || (profile.Username != "" && profile.Password != ""):
				// Nothing given: keep the stored credentials.
			default:
				if !interactive {
					return fmt.Errorf("--token or --username/--password is required")
				}
				if username, err = prompt(in, out, "username", true, profile.Username); err != nil {
					return err
				}
				if password, err = promptSecret(in, out, "password"); err != nil {
					return err
				}
				profile.Token, profile.Username, profile.Password = "", username, password
			}

			if cmd.Flags().Changed("insecure") {
				profile.Insecure = insecure
			}

			// Only the first profile becomes current on its own; switching
			// is explicit ('xo configure use').
			makeCurrent := len(profiles) == 0
			path, err := config.Upsert(profile, makeCurrent)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "Profile %q saved to %s\n", profile.Name, path); err != nil {
				return err
			}
			if current, _, err := config.List(); err == nil && current != profile.Name {
				if _, err := fmt.Fprintf(out, "It is not the current profile (%q): use --profile %s, or 'xo configure use %s'.\n", current, profile.Name, profile.Name); err != nil {
					return err
				}
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
	cmd.AddCommand(newUseCommand())

	return cmd
}

// selectedProfile returns the profile a configure command works on, with
// the same precedence as every other command: --profile, then
// $XOA_PROFILE, then the current profile, then "default".
func selectedProfile(cmd *cobra.Command) (string, error) {
	if name := strings.TrimSpace(cli.ProfileName(cmd)); name != "" {
		return name, nil
	}
	if name := strings.TrimSpace(os.Getenv(config.EnvProfile)); name != "" {
		return name, nil
	}
	current, _, err := config.List()
	if err != nil {
		return "", err
	}
	return current, nil
}

func isTerminal(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

// prompt reads a value. A non-empty def is the stored value: it is shown in
// brackets and kept when the answer is empty, and used as is when stdin is
// not a terminal.
func prompt(in io.Reader, out io.Writer, label string, interactive bool, def string) (string, error) {
	if !interactive {
		if def != "" {
			return def, nil
		}
		return "", fmt.Errorf("--%s is required (or set the corresponding XOA_ environment variable)", label)
	}
	question := label + ": "
	if def != "" {
		question = fmt.Sprintf("%s [%s]: ", label, def)
	}
	if _, err := fmt.Fprint(out, question); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" && def == "" {
		return "", fmt.Errorf("cannot read %s: %w", label, err)
	}
	if answer := strings.TrimSpace(line); answer != "" {
		return answer, nil
	}
	return def, nil
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
