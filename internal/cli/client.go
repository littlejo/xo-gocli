package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/config"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"

	xov2 "github.com/vatesfr/xenorchestra-go-sdk/v2"
	v2client "github.com/vatesfr/xenorchestra-go-sdk/v2/client"

	xoconfig "github.com/littlejo/xo-gocli/internal/config"
	"github.com/littlejo/xo-gocli/internal/output"
)

const (
	// FlagProfile is the global --profile flag.
	FlagProfile = "profile"
	// FlagOutput is the global --output flag.
	FlagOutput = "output"
	// FlagJSON is the global --json flag: a boolean shortcut that selects
	// JSON output, for AWS CLI familiarity. An explicit --output still wins.
	FlagJSON = "json"
	// FlagDebug is the global --debug flag.
	FlagDebug = "debug"
	// FlagTimeout is the global --timeout flag.
	FlagTimeout = "timeout"
	// EnvDebug enables verbose SDK/API error diagnostics (like --debug).
	EnvDebug = "XOA_DEBUG"
	// EnvTimeout is the script counterpart of --timeout (a Go duration like
	// "60s" or "2m").
	EnvTimeout = "XOA_TIMEOUT"
	// EnvWait is the script counterpart of the --wait flag of the
	// asynchronous actions (a "1", "true" or "yes" value).
	EnvWait = "XOA_WAIT"
)

// defaultClientTimeout is the HTTP client timeout when neither --timeout nor
// $XOA_TIMEOUT is given. It matches the SDK's own default (client.New falls
// back to 30 s when ClientTimeout is zero); the flag simply makes it explicit
// and overridable per invocation.
const defaultClientTimeout = 30 * time.Second

// Version is set at build time with -ldflags "-X ...=x.y.z".
var Version = "dev"

// Execute runs the given root command with the provided context and returns
// the first error encountered, if any.
//
// The command's stderr is used for diagnostics: when --debug (or $XOA_DEBUG)
// is set, extra context is printed about any failure — which profile and
// endpoint resolved, and the raw SDK/API error carried by the command.
// Normal runs print nothing extra; the single "Error: …" line is emitted by
// the caller (main).
func Execute(ctx context.Context, root *cobra.Command, args []string) error {
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return nil
	}
	if Debug(root) {
		printDebugError(root.ErrOrStderr(), root, err)
	}
	return err
}

// Debug reports whether verbose SDK/API error diagnostics are requested,
// either through the global --debug flag or the $XOA_DEBUG environment
// variable. The flag wins; the variable uses the same truthy values as
// XOA_YES (1, true, yes, case-insensitive).
func Debug(cmd *cobra.Command) bool {
	if cmd != nil {
		if b, err := cmd.Root().PersistentFlags().GetBool(FlagDebug); err == nil && b {
			return true
		}
	}
	v := os.Getenv(EnvDebug)
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// detailError wraps an error with an extra diagnostic detail. The detail is
// not part of Error() (so normal output stays concise and stable for
// scripting); it is revealed only in debug mode, where Execute prints it.
type detailError struct {
	err    error
	detail string
}

func (e *detailError) Error() string { return e.err.Error() }
func (e *detailError) Unwrap() error { return e.err }

// Detail returns the hidden diagnostic detail, or "" if err carries none.
func Detail(err error) string {
	var de *detailError
	if errors.As(err, &de) {
		return de.detail
	}
	return ""
}

// NotFound turns a lookup failure into the concise "<kind> not found" message
// when the API returned a 404 — detected on the actual HTTP status carried by
// the SDK error, not a substring of the message — keeping the raw SDK error
// as debug-only detail (see --debug / $XOA_DEBUG). For any other error it
// translates the SDK/Go internals to a concise message (see translateSDKError)
// and wraps it in "cannot <verb> <kind> <id>: …" with the TLS hint.
//
// verb is the verb shown for non-404 failures ("get" for a read, "resolve"
// for an existence check); it does not affect the 404 wording.
func NotFound(kind, verb, id string, err error, insecure bool) error {
	if err != nil && APIStatus(err) == http.StatusNotFound {
		return &detailError{
			err:    fmt.Errorf("%s %q not found", kind, id),
			detail: err.Error(),
		}
	}
	return InsecureHint(fmt.Sprintf("cannot %s %s %q: %v", verb, kind, id, err), insecure)
}

// printDebugError prints the --debug diagnostics for a failing run to w:
// which profile and endpoint resolved (best effort, never failing) and, when
// the command attached one, the raw SDK/API error behind the concise message
// (e.g. the "API error: 404 Not Found - …" line for a not-found lookup).
func printDebugError(w io.Writer, cmd *cobra.Command, err error) {
	if cfg, errLoad := xoconfig.Load(ProfileName(cmd)); errLoad == nil {
		_, _ = fmt.Fprintf(w, "debug: profile=%s endpoint=%s\n", cfg.Name, cfg.Endpoint)
	}
	if d := Detail(err); d != "" {
		_, _ = fmt.Fprintf(w, "debug: %s\n", d)
	}
}

// ProfileName returns the profile selected with --profile or $XOA_PROFILE.
func ProfileName(cmd *cobra.Command) string {
	name, _ := cmd.Flags().GetString(FlagProfile)
	return name
}

// OutputFormat returns the output format selected for this run.
// Precedence, highest first:
//
//  1. the --output flag when explicitly given by the user;
//  2. the --json flag (a boolean shortcut that selects JSON output);
//  3. the $XOA_DEFAULT_OUTPUT environment variable;
//  4. the "output" value stored on the active profile;
//  5. table, the human friendly default.
//
// Steps 3 and 4 need the profile to load; when it cannot (for example an
// unconfigured command), the chain silently falls back to the default.
// The returned name is validated at render time by output.ParseFormat.
func OutputFormat(cmd *cobra.Command) string {
	if cmd == nil {
		return string(output.FormatTable)
	}

	if f, err := cmd.Flags().GetString(FlagOutput); err == nil && cmd.Flags().Changed(FlagOutput) {
		return f
	}
	if b, err := cmd.Root().PersistentFlags().GetBool(FlagJSON); err == nil && b {
		return string(output.FormatJSON)
	}
	if v := os.Getenv(xoconfig.EnvDefaultOutput); v != "" {
		return v
	}
	if cfg, err := xoconfig.Load(ProfileName(cmd)); err == nil && cfg.Output != "" {
		return cfg.Output
	}
	return string(output.FormatTable)
}

// Timeout returns the HTTP client timeout for this invocation. Precedence:
// the --timeout flag, then the $XOA_TIMEOUT environment variable (a Go
// duration such as "60s" or "2m"), then the 30-second default. It is applied
// to the SDK's ClientTimeout (see buildSDKConfig) so every request — and, for
// long-running operations such as a pool rolling update, the whole task wait —
// can be raised above the SDK's hard-coded 30-second floor.
func Timeout(cmd *cobra.Command) time.Duration {
	var d time.Duration
	if cmd != nil {
		if v, err := cmd.Root().PersistentFlags().GetDuration(FlagTimeout); err == nil && v > 0 {
			d = v
		}
	}
	if d == 0 {
		if v := os.Getenv(EnvTimeout); v != "" {
			if parsed, err := time.ParseDuration(v); err == nil {
				d = parsed
			}
		}
	}
	if d == 0 {
		d = defaultClientTimeout
	}
	// Remember the value that will actually be in effect for the requests of
	// this invocation, so error messages can report it ("timed out after …").
	currentClientTimeout = d
	return d
}

// SkipConfirm reports whether destructive operations should run without a
// confirmation prompt: either the --yes flag or the $XOA_YES environment
// variable. The variable exists so scripts and CI pipelines can confirm
// non-interactively without repeating --yes on every command; it is checked
// only by confirmations, so setting it has no other effect.
func SkipConfirm(cmd *cobra.Command) bool {
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return true
	}
	if v := os.Getenv("XOA_YES"); v != "" {
		return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	}
	return false
}

// WaitEnabled reports whether an asynchronous action should wait for its task
// to complete: either the --wait flag or the $XOA_WAIT environment variable.
// The variable exists so scripts and CI pipelines can wait without repeating
// --wait on every command (e.g. `XOA_WAIT=1 xo vm start <id>`); it only
// affects the --wait flag, not the wait deadline of 'xo task wait'.
func WaitEnabled(cmd *cobra.Command) bool {
	if wait, _ := cmd.Flags().GetBool("wait"); wait {
		return true
	}
	if v := os.Getenv(EnvWait); v != "" {
		return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	}
	return false
}

// NewClient resolves the selected profile and builds an authenticated SDK v2
// client. Commands must pass their cobra context to the SDK operations so
// that cancellation (Ctrl+C) reaches the HTTP layer; the SDK client enforces
// its own per-request timeout, set from --timeout / $XOA_TIMEOUT (Timeout).
func NewClient(cmd *cobra.Command, cfg *xoconfig.ClientConfig) (library.Library, error) {
	sdkConfig, err := buildSDKConfig(cfg, Timeout(cmd))
	if err != nil {
		return nil, err
	}

	client, err := xov2.New(sdkConfig)
	if err != nil {
		return nil, newConnectionError(cfg, err)
	}
	return client, nil
}

// NewHTTPClient builds the authenticated SDK v2 REST client.
//
// The v2 client exposes its HttpClient, BaseURL and AuthToken specifically so
// callers can talk to REST endpoints the SDK does not (yet) wrap with a typed
// service. It is the same single API boundary as NewClient: authentication,
// TLS and base URL handling all come from the SDK.
//
// Note: for resources the SDK already exposes, prefer NewClient and the typed
// service; use this only for endpoints that are a known gap in the SDK (the
// missing operation should be contributed upstream).
func NewHTTPClient(cmd *cobra.Command, cfg *xoconfig.ClientConfig) (*v2client.Client, error) {
	sdkConfig, err := buildSDKConfig(cfg, Timeout(cmd))
	if err != nil {
		return nil, err
	}
	client, err := v2client.New(sdkConfig)
	if err != nil {
		return nil, newConnectionError(cfg, err)
	}
	return client, nil
}

func newConnectionError(cfg *xoconfig.ClientConfig, err error) error {
	// A transport failure while building the client (or logging in) is
	// translated like any other SDK error; the raw text is kept as detail.
	if translated, raw := translateSDKError(err.Error()); translated != "" {
		return &detailError{err: errors.New(translated), detail: raw}
	}
	var msg string
	if cfg.Token == "" && cfg.Username != "" {
		msg = fmt.Sprintf("authentication to %s failed: %v", cfg.Endpoint, err)
	} else {
		msg = fmt.Sprintf("cannot connect to %s: %v", cfg.Endpoint, err)
	}
	return InsecureHint(msg, cfg.Insecure)
}

// InsecureHint returns a concise error for msg, appending a hint when the
// failure is a TLS certificate problem and insecure mode is not already
// enabled.
//
// Before that, msg is run through translateSDKError: known SDK/Go error forms
// (timeout, unreachable endpoint, malformed API response, 401/403) are
// replaced by a concise message, and the original text is kept as the
// debug-only detail that Execute reveals with --debug. Messages that match no
// known form (including local I/O errors and "API error: 5xx" lines carrying
// the server's own body) are returned unchanged.
func InsecureHint(msg string, alreadyInsecure bool) error {
	msg = cleanSDKArtifact(msg)
	if translated, raw := translateSDKError(msg); translated != "" {
		return &detailError{err: errors.New(translated), detail: raw}
	}
	if alreadyInsecure || !isTLSVerifyError(msg) {
		return errors.New(msg)
	}
	return errors.New(msg + "\n\nthe server certificate could not be verified; if this is a self-signed or internal certificate, retry with 'xo configure --insecure' (or set XOA_INSECURE=1)")
}

// cleanSDKArtifact removes the trailing "%!(EXTRA ...)" marker that the Go
// runtime appends when the SDK formats an error with more arguments than
// format verbs. It is purely cosmetic and always appears at the end.
func cleanSDKArtifact(msg string) string {
	if i := strings.Index(msg, "%!(EXTRA"); i >= 0 {
		return strings.TrimSpace(msg[:i])
	}
	return msg
}

func isTLSVerifyError(msg string) bool {
	return strings.Contains(msg, "certificate") || strings.Contains(msg, "x509")
}

// buildSDKConfig translates the resolved profile plus the per-invocation
// HTTP client timeout into the SDK v2 configuration. The timeout is applied
// to the SDK's ClientTimeout (its HTTP client), which bounds every request
// and, for long-running operations, the whole task wait. A value of zero
// keeps the SDK's own 30-second default; the CLI always passes an explicit
// duration (see Timeout), so the floor is only ever raised, never lowered.
func buildSDKConfig(cfg *xoconfig.ClientConfig, clientTimeout time.Duration) (*config.Config, error) {
	sdk := &config.Config{
		Url:                cfg.Endpoint,
		Token:              cfg.Token,
		Username:           cfg.Username,
		Password:           cfg.Password,
		InsecureSkipVerify: cfg.Insecure,
		ClientTimeout:      clientTimeout,
		// Keep the SDK quiet: the CLI owns stdout/stderr.
		LogOutputPaths:      []string{"/dev/null"},
		LogErrorOutputPaths: []string{"/dev/null"},
	}
	return config.NewWithValues(sdk)
}
