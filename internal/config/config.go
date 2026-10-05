// Package config implements CLI-level configuration: named profiles,
// environment variables and the configuration file.
//
// It only stores connection settings. Authentication and HTTP handling are
// delegated to the Xen Orchestra SDK.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// EnvProfile selects the active profile (like AWS_PROFILE).
	EnvProfile = "XOA_PROFILE"
	// EnvEndpoint overrides the profile endpoint.
	EnvEndpoint = "XOA_ENDPOINT"
	// EnvToken overrides the profile authentication token.
	EnvToken = "XOA_TOKEN"
	// EnvUsername overrides the profile username.
	EnvUsername = "XOA_USERNAME"
	// EnvPassword overrides the profile password.
	EnvPassword = "XOA_PASSWORD"
	// EnvInsecure overrides the profile insecure flag.
	EnvInsecure = "XOA_INSECURE"
	// EnvDefaultOutput is the script counterpart of the per-profile
	// "output" setting: it selects the default output format (table, json,
	// yaml or text) without passing --output on every command.
	EnvDefaultOutput = "XOA_DEFAULT_OUTPUT"
)

// DefaultProfile is used when no profile is selected.
const DefaultProfile = "default"

// Profile holds the connection settings for one Xen Orchestra instance.
type Profile struct {
	Name     string `yaml:"name" json:"name"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	Token    string `yaml:"token" json:"token,omitempty"`
	Username string `yaml:"username" json:"username,omitempty"`
	Password string `yaml:"password" json:"password,omitempty"`
	Insecure bool   `yaml:"insecure" json:"insecure"`
	// Output is the default output format for commands run against this
	// profile (table, json, yaml or text). A per-invocation --output flag
	// always wins over it.
	Output string `yaml:"output,omitempty" json:"output,omitempty"`
}

// File is the on-disk representation of the configuration.
type File struct {
	Current  string    `yaml:"current"`
	Profiles []Profile `yaml:"profiles"`
}

// ClientConfig is the resolved configuration for a single command run.
type ClientConfig struct {
	Name     string
	Endpoint string
	Token    string
	Username string
	Password string
	Insecure bool
	// Output is the default output format resolved for this run (the
	// profile's stored value, overridden by $XOA_DEFAULT_OUTPUT). It is
	// never validated here: the format is checked at render time, where a
	// per-invocation --output flag may have replaced it.
	Output string
}

// Load reads the configuration file, applies environment overrides and
// returns the resolved profile named profileName.
//
// Profile selection follows the AWS CLI precedence: an explicit profile name
// (flag) wins, then $XOA_PROFILE, then the file's current profile, then the
// default profile.
func Load(profileName string) (*ClientConfig, error) {
	file, err := read()
	if err != nil {
		return nil, err
	}

	if profileName == "" {
		profileName = os.Getenv(EnvProfile)
	}
	if profileName == "" {
		profileName = file.Current
	}
	if profileName == "" {
		profileName = DefaultProfile
	}

	profile := findProfile(file, profileName)
	if profile == nil && profileName != DefaultProfile {
		return nil, fmt.Errorf("profile %q is not configured, run 'xo configure --profile %s'", profileName, profileName)
	}

	cfg := &ClientConfig{Name: profileName}
	if profile != nil {
		cfg.Endpoint = profile.Endpoint
		cfg.Token = profile.Token
		cfg.Username = profile.Username
		cfg.Password = profile.Password
		cfg.Insecure = profile.Insecure
		cfg.Output = profile.Output
	}

	cfg.applyEnv()

	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("no endpoint configured for profile %q, set the %s environment variable or run 'xo configure'", profileName, EnvEndpoint)
	}
	if cfg.Token == "" && (cfg.Username == "" || cfg.Password == "") {
		return nil, fmt.Errorf("no credentials configured for profile %q, run 'xo configure' or set %s (or %s and %s)", profileName, EnvToken, EnvUsername, EnvPassword)
	}

	return cfg, nil
}

func findProfile(file *File, name string) *Profile {
	for i := range file.Profiles {
		if file.Profiles[i].Name == name {
			return &file.Profiles[i]
		}
	}
	return nil
}

func (c *ClientConfig) applyEnv() {
	if v := os.Getenv(EnvEndpoint); v != "" {
		c.Endpoint = v
	}
	if v := os.Getenv(EnvToken); v != "" {
		c.Token = v
	}
	if v := os.Getenv(EnvUsername); v != "" {
		c.Username = v
	}
	if v := os.Getenv(EnvPassword); v != "" {
		c.Password = v
	}
	if v := os.Getenv(EnvInsecure); v != "" {
		c.Insecure = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv(EnvDefaultOutput); v != "" {
		c.Output = v
	}
}

// read parses the configuration file. A missing file is not an error.
func read() (*File, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &File{Current: DefaultProfile}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read configuration file %s: %w", path, err)
	}

	file := &File{Current: DefaultProfile}
	if err := yaml.Unmarshal(data, file); err != nil {
		return nil, fmt.Errorf("cannot parse configuration file %s: %w", path, err)
	}
	return file, nil
}

// WriteFile stores profiles in the configuration file, creating it (and its
// directory) when needed. The file is written with 0600 permissions because it
// may contain credentials.
func WriteFile(file *File) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	data, err := yaml.Marshal(file)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("cannot create configuration directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("cannot write configuration file: %w", err)
	}
	return path, nil
}

// Upsert merges profile into file (matching by name) and updates the current
// profile marker. It returns the path of the written file.
func Upsert(profile Profile, makeCurrent bool) (string, error) {
	file, err := read()
	if err != nil {
		return "", err
	}

	found := false
	for i := range file.Profiles {
		if file.Profiles[i].Name == profile.Name {
			file.Profiles[i] = profile
			found = true
			break
		}
	}
	if !found {
		file.Profiles = append(file.Profiles, profile)
	}
	if makeCurrent {
		file.Current = profile.Name
	}
	return WriteFile(file)
}

// List returns the stored profiles and the name of the current profile. The
// credentials are returned verbatim so the caller can mask them.
func List() (current string, profiles []Profile, err error) {
	file, err := read()
	if err != nil {
		return "", nil, err
	}
	if file.Current == "" {
		file.Current = DefaultProfile
	}
	return file.Current, file.Profiles, nil
}

// GetProfile returns the stored profile named name.
func GetProfile(name string) (*Profile, error) {
	file, err := read()
	if err != nil {
		return nil, err
	}
	profile := findProfile(file, name)
	if profile == nil {
		return nil, fmt.Errorf("profile %q is not configured, run 'xo configure --profile %s'", name, name)
	}
	return profile, nil
}

// Remove deletes the profile named name from the configuration file. If it was
// the current profile, current falls back to the first remaining profile (or
// the default profile when none remain).
func Remove(name string) (string, error) {
	file, err := read()
	if err != nil {
		return "", err
	}
	kept := file.Profiles[:0]
	found := false
	for _, p := range file.Profiles {
		if p.Name == name {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	if !found {
		return "", fmt.Errorf("profile %q is not configured, run 'xo configure --profile %s'", name, name)
	}
	file.Profiles = kept

	if file.Current == name {
		if len(kept) > 0 {
			file.Current = kept[0].Name
		} else {
			file.Current = DefaultProfile
		}
	}
	if _, err := WriteFile(file); err != nil {
		return "", err
	}
	return file.Current, nil
}

// Path returns the location of the configuration file, honoring the
// XOA_CONFIG_FILE environment variable used by the tests.
func Path() (string, error) {
	if p := os.Getenv("XOA_CONFIG_FILE"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "xo", "config"), nil
}
