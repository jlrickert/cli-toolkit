package env

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// OsEnv is an Env implementation that delegates to the real process
// environment. Use this in production code where access to the actual OS
// environment is required.
type OsEnv struct {
	jail string
}

func cleanJail(jail string) string {
	if strings.TrimSpace(jail) == "" {
		return ""
	}
	return filepath.Clean(jail)
}

func (o *OsEnv) Name() string {
	return "os"
}

func (o *OsEnv) GetJail() string {
	return o.jail
}

func (o *OsEnv) SetJail(jail string) error {
	o.jail = cleanJail(jail)
	return nil
}

// GetHome returns the home directory reported by the OS. It delegates to
// os.UserHomeDir.
func (o *OsEnv) GetHome() (string, error) {
	return os.UserHomeDir()
}

// SetHome sets environment values that represent the user's home directory.
//
// On Windows it also sets USERPROFILE to satisfy common callers.
func (o *OsEnv) SetHome(home string) error {
	if runtime.GOOS == "windows" {
		if err := os.Setenv("USERPROFILE", home); err != nil {
			return err
		}
	}
	return os.Setenv("HOME", home)
}

// GetUser returns the current OS user username. If the Username field is
// empty it falls back to the user's Name field.
func (o *OsEnv) GetUser() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	// Username is typically what callers expect; fall back to Name if empty.
	if u.Username != "" {
		return u.Username, nil
	}
	return u.Name, nil
}

// SetUser sets environment values that represent the current user.
//
// On Windows it also sets USERNAME in addition to USER.
func (o *OsEnv) SetUser(username string) error {
	if runtime.GOOS == "windows" {
		if err := os.Setenv("USERNAME", username); err != nil {
			return err
		}
	}
	return os.Setenv("USER", username)
}

// Get returns the environment variable for key.
func (o *OsEnv) Get(key string) string {
	return os.Getenv(key)
}

// Set sets the OS environment variable key to value.
func (o *OsEnv) Set(key string, value string) error {
	return os.Setenv(key, value)
}

// Environ returns a copy of the process environment in "key=value" form.
func (o *OsEnv) Environ() []string {
	return os.Environ()
}

// Has reports whether the given environment key is present.
func (o *OsEnv) Has(key string) bool {
	_, ok := os.LookupEnv(key)
	return ok
}

// Unset removes the OS environment variable.
func (o *OsEnv) Unset(key string) {
	_ = os.Unsetenv(key)
}

// GetTempDir returns the OS temporary directory.
func (o *OsEnv) GetTempDir() string {
	return os.TempDir()
}

// Getwd returns the current process working directory.
func (o *OsEnv) Getwd() (string, error) {
	return os.Getwd()
}

// Setwd attempts to change the process working directory to dir.
//
// It also attempts to update PWD.
func (o *OsEnv) Setwd(dir string) error {
	p, _ := filepath.Abs(dir)
	return os.Chdir(p)
}

func (o *OsEnv) CloneEnv() Env {
	jailPath := o.jail
	return &OsEnv{jail: jailPath}
}

func (o *OsEnv) ExpandPath(p string) string {
	if p == "" {
		return p
	}
	if p[0] != '~' {
		return p
	}

	// Only expand the simple leading tilde forms: "~" or "~/" or "~\".
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, _ := o.GetHome()
		if p == "~" {
			return filepath.Clean(home)
		}
		// Trim the "~/" or "~\" prefix and join with home to produce a
		// well formed path.
		rest := p[2:]
		return filepath.Join(home, rest)
	}

	// More complex cases like "~username/..." are not supported and are
	// returned unchanged.
	return p
}

// Ensure implementations satisfy the interfaces.
var _ Env = (*OsEnv)(nil)
