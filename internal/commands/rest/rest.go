// Package rest implements the 'xo rest' command: a low-level REST escape
// hatch for talking to any Xen Orchestra REST endpoint the SDK v2 does not (yet)
// wrap in a typed service.
//
// It is not a second REST client. The request goes through the SDK v2 client's
// own HTTP facilities — the authenticated http.Client, the resolved base URL
// (/rest/v0) and the authentication token — built by cli.NewHTTPClient. This is
// the same single API boundary used by 'xo token' and is the usage the SDK
// explicitly documents for "missing endpoints". A missing typed service reached
// this way should be contributed to the SDK.
//
// For resources the SDK already exposes, prefer the typed commands
// ('xo vm list', 'xo host get', …); use 'xo rest' only for the gap.
package rest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/vatesfr/xenorchestra-go-sdk/v2/client"

	"github.com/littlejo/xo-gocli/internal/cli"
	"github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	flagData    = "data"
	flagParam   = "param"
	flagHeader  = "header"
	flagQuery   = "query"
	flagYes     = "yes"
	flagInclude = "include"

	// authCookieName is the cookie the SDK v2 client uses to carry the token.
	authCookieName = "authenticationToken"
)

// methods is the set of HTTP methods the escape hatch supports, in the order
// shown in help output.
var methods = []string{"get", "post", "put", "patch", "delete"}

var methodSet = map[string]bool{
	http.MethodGet:    true,
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

// bodyMethods are the methods that may carry a request body.
var bodyMethods = map[string]bool{
	http.MethodPost:  true,
	http.MethodPut:   true,
	http.MethodPatch: true,
}

// NewCommand builds the 'xo rest' command.
func NewCommand() *cobra.Command {
	var (
		data    string
		params  []string
		headers []string
		query   string
		include bool
	)

	cmd := &cobra.Command{
		Use:   "rest <method> <path>",
		Short: "Call a raw Xen Orchestra REST endpoint",
		Long: `Call a raw Xen Orchestra REST endpoint.

This is an escape hatch for endpoints that have no typed command yet (users
and groups, SR actions, ...), not a duplicate of the typed commands: if
'xo vm list' covers it, use 'xo vm list'. The request is sent
through the SDK v2 HTTP client (same authentication, base URL and TLS
handling), so this is not a second REST client. The OpenAPI spec at
<endpoint>/rest/v0/docs lists every endpoint and its fields; the REST API
documentation lives at https://docs.xen-orchestra.com/automation/restapi.

The <path> is relative to the REST API root (/rest/v0), so 'vdis' hits
/rest/v0/vdis.

Methods: get, post, put, patch, delete.
The request body (for post/put/patch) comes from --data as JSON; use --data -
to read it from stdin.

'xo rest delete' asks for confirmation; pass --yes to run non-interactively.

Examples:
  xo rest get vdis --output json  # raw VDI fields (typed: 'xo vdi list')
  xo rest get vdis --param limit=10
  xo rest get vdis/<id>
  xo rest post srs/<id>/actions/scan       # action without a typed command
  xo rest get users --output json          # users have no typed command yet
  xo rest get vdis --output json --query '[].name_label'
  xo rest get vdis -i             # also print the status line and headers on stderr`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			endpoint := args[1]
			if !methodSet[method] {
				return fmt.Errorf("unsupported method %q (expected one of: %s)", args[0], strings.Join(methods, ", "))
			}
			if strings.TrimSpace(endpoint) == "" {
				return fmt.Errorf("path is required")
			}
			if err := output.ValidateQuery(query); err != nil {
				return err
			}
			format, err := output.ParseFormat(cli.OutputFormat(cmd))
			if err != nil {
				return err
			}

			takesBody := bodyMethods[method]
			if data != "" && !takesBody {
				return fmt.Errorf("--data is not valid with %s", strings.ToLower(method))
			}
			var body []byte
			if data != "" {
				body, err = parseData(data, cmd.InOrStdin())
				if err != nil {
					return err
				}
			}
			queryParams, err := parseParams(params)
			if err != nil {
				return err
			}
			extraHeaders, err := parseHeaders(headers)
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

			if method == http.MethodDelete && !cli.SkipConfirm(cmd) {
				ok, err := confirm(cmd, fmt.Sprintf("Are you sure you want to DELETE %s?", endpoint))
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("aborted")
				}
			}

			statusLine, respHeaders, respBody, err := doRestRequest(cmd.Context(), httpClient, method, endpoint, body, queryParams, extraHeaders)
			if err != nil {
				return cli.InsecureHint(fmt.Sprintf("rest %s %s: %v", strings.ToLower(method), endpoint, err), cfg.Insecure)
			}

			if include {
				if err := renderInclude(cmd.ErrOrStderr(), statusLine, respHeaders); err != nil {
					return err
				}
			}

			var raw any
			if len(bytes.TrimSpace(respBody)) > 0 {
				if err := json.Unmarshal(respBody, &raw); err != nil {
					// Non-JSON body: surface it verbatim as a string.
					raw = string(respBody)
				}
			}

			return renderRaw(cmd.OutOrStdout(), format, raw, query)
		},
	}

	flags := cmd.Flags()
	// No shorthand for --data: the root owns -d for --debug, and a
	// shorthand collision in the merged flagset makes cobra panic on
	// every 'xo rest' invocation (see the regression test in
	// internal/commands/commands_test.go).
	flags.StringVar(&data, flagData, "", `JSON request body for post/put/patch ('-' reads stdin)`)
	flags.StringSliceVar(&params, flagParam, nil, `query parameter KEY=VALUE (repeatable)`)
	flags.StringSliceVar(&headers, flagHeader, nil, `extra request header KEY: VALUE (repeatable)`)
	flags.StringVarP(&query, flagQuery, "q", "", "JMESPath expression applied to the result, e.g. '[].name_label'")
	// --yes (or $XOA_YES) skips the confirmation; it is read via
	// cli.SkipConfirm, which sees both the flag and the environment variable.
	flags.Bool(flagYes, false, "do not ask for confirmation (or set XOA_YES=1)")
	flags.BoolVarP(&include, flagInclude, "i", false, "print the status line and response headers on stderr")

	return cmd
}

// doRestRequest performs the request through the SDK v2 client's HTTP
// facilities (authenticated client, /rest/v0 base URL and auth token). It
// returns the response status line, headers and body. For non-2xx responses the
// body is folded into the error.
func doRestRequest(ctx context.Context, c *client.Client, method, endpoint string,
	body []byte, queryParams url.Values, headers http.Header) (string, http.Header, []byte, error) {
	u := *c.BaseURL
	u.Path = path.Join(c.BaseURL.Path, strings.TrimPrefix(endpoint, "/"))
	if len(queryParams) > 0 {
		u.RawQuery = queryParams.Encode()
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return "", nil, nil, err
	}

	// User headers are applied first so they may override the defaults.
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: string(c.AuthToken)})

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return "", nil, nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if cerr := resp.Body.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return resp.Status, nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.Status, resp.Header, nil, apiError(resp.Status, string(respBody))
	}
	return resp.Status, resp.Header, respBody, nil
}

// apiError formats a non-2xx response as a concise error, keeping the response
// body when it carries a useful message.
func apiError(status, body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("API error: %s", status)
	}
	return fmt.Errorf("API error: %s - %s", status, body)
}

// parseData reads and validates the request body. An empty value yields no
// body. '-' reads the body from in. The body must be valid JSON; it is
// normalized (re-marshalled) so the request always carries well-formed JSON.
func parseData(data string, in io.Reader) ([]byte, error) {
	var src []byte
	if data == "-" {
		b, err := io.ReadAll(in)
		if err != nil {
			return nil, fmt.Errorf("cannot read body from stdin: %v", err)
		}
		src = b
	} else {
		src = []byte(data)
	}
	if len(bytes.TrimSpace(src)) == 0 {
		return nil, nil
	}
	var decoded any
	if err := json.Unmarshal(src, &decoded); err != nil {
		return nil, fmt.Errorf("invalid --data: expected JSON, got: %v", err)
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("invalid --data: %v", err)
	}
	return encoded, nil
}

// parseParams turns repeated --param KEY=VALUE flags into URL query values.
func parseParams(params []string) (url.Values, error) {
	values := url.Values{}
	for _, p := range params {
		key, value, found := strings.Cut(p, "=")
		if !found {
			return nil, fmt.Errorf("invalid --param %q (expected KEY=VALUE)", p)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("invalid --param %q (expected KEY=VALUE)", p)
		}
		values.Add(key, value)
	}
	return values, nil
}

// parseHeaders turns repeated --header 'KEY: VALUE' flags into an http.Header.
// Both ':' and '=' are accepted as separators.
func parseHeaders(headers []string) (http.Header, error) {
	h := http.Header{}
	for _, header := range headers {
		key, value, found := strings.Cut(header, ":")
		if !found {
			key, value, found = strings.Cut(header, "=")
		}
		if !found || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid --header %q (expected KEY: VALUE)", header)
		}
		h.Set(strings.TrimSpace(key), strings.TrimSpace(value))
	}
	return h, nil
}

// renderRaw applies the optional --query expression and renders the result in
// the requested format. For the default (table) and text formats the raw
// payload is rendered generically: a list of objects becomes an auto-column
// table, an object becomes key/value lines, a scalar is printed as-is.
func renderRaw(w io.Writer, format output.Format, raw any, query string) error {
	if query != "" {
		queryResult, err := output.Query(query, raw)
		if err != nil {
			return err
		}
		return output.Render(w, format, output.Table{}, raw, queryResult)
	}
	if format == output.FormatJSON || format == output.FormatYAML {
		return output.Render(w, format, output.Table{}, raw, nil)
	}
	return output.Render(w, output.FormatText, output.Table{}, raw, nil)
}

// renderInclude writes the status line and response headers to w (stderr), so
// the body on stdout stays clean for scripting.
func renderInclude(w io.Writer, statusLine string, headers http.Header) error {
	if _, err := fmt.Fprintf(w, "HTTP/1.1 %s\n", statusLine); err != nil {
		return err
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	// Stable order for predictable output.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	for _, key := range keys {
		for _, value := range headers[key] {
			if _, err := fmt.Fprintf(w, "%s: %s\n", key, value); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

// confirm asks the user to confirm a destructive operation. When stdin is not
// a terminal it refuses so that automation never blocks, forcing --yes.
func confirm(cmd *cobra.Command, message string) (bool, error) {
	in := cmd.InOrStdin()
	if !isTerminal(in) {
		return false, fmt.Errorf("confirmation required: re-run with --yes to proceed non-interactively")
	}
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N]: ", message); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	if err != nil && answer == "" {
		return false, fmt.Errorf("cannot read confirmation: %v", err)
	}
	return answer == "y" || answer == "yes", nil
}

// isTerminal reports whether in is connected to a terminal.
func isTerminal(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}
