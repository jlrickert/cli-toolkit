package env

import (
	"maps"
	"os"
	"sort"
	"strings"

	"github.com/jlrickert/cli-toolkit/toolkit/jail"
)

// Env is a compact interface for reading and modifying environment
// values. Implementations may reflect the real process environment (OsEnv)
// or provide an in-memory view suitable for tests (TestEnv).
//
// The interface mirrors common environment operations so callers can inject a
// test implementation for unit tests without touching the real process env.
type Env interface {
	jail.Jailed

	// Name of the environment
	Name() string

	// Get returns the raw environment value for key. The return value may be
	// empty when the key is not present.
	Get(key string) string

	// Set assigns the environment key to value.
	Set(key, value string) error

	// Has reports whether the environment key is set.
	Has(key string) bool

	// Environ returns a copy of the environment as a slice of strings in the
	// form "KEY=VALUE".
	Environ() []string

	// Unset removes the environment key.
	Unset(key string)

	// GetHome returns the user's home directory. Implementations should return
	// an error if the value is not available.
	GetHome() (string, error)

	// SetHome sets the user's home directory in the environment.
	SetHome(home string) error

	// GetUser returns the current user's username. Implementations should
	// return an error if the value is not available.
	GetUser() (string, error)

	// SetUser sets the current user's username in the environment.
	SetUser(user string) error

	// Getwd returns the working directory as seen by this Env. For OsEnv this
	// is the process working directory; for TestEnv it is the stored PWD.
	Getwd() (string, error)

	// Setwd sets the working directory for this Env. For OsEnv this may change
	// the process working directory; for TestEnv it updates the stored PWD.
	Setwd(dir string) error

	// GetTempDir returns an appropriate temp directory for this Env. For OsEnv
	// this delegates to os.TempDir(); TestEnv provides testable fallbacks.
	GetTempDir() string
}

// EnvCloner is implemented by environments that can create a deep-copy clone
// suitable for isolated mutation in concurrent runs.
type EnvCloner interface {
	CloneEnv() Env
}

// GetDefault returns the value of key from env when present and non-empty.
// Otherwise it returns the provided fallback value. Use this helper when a
// preference for an environment value is desired while still allowing a
// concrete default.
func GetDefault(env Env, key, other string) string {
	if env == nil {
		return other
	}
	if v := env.Get(key); v != "" {
		return v
	}
	return other
}

// ExpandEnv expands $var or ${var} in s using the provided env. If env is nil,
// the real OS environment is used via OsEnv.
func ExpandEnv(env Env, s string) string {
	if env == nil {
		env = &OsEnv{}
	}
	return os.Expand(s, env.Get)
}

// DumpEnv returns a sorted, newline separated representation of the
// environment visible via env. Each line is formatted as "KEY=VALUE".
//
// For TestEnv and OsEnv the function enumerates the known keys. For other Env
// implementations the function attempts to use common helper methods (Environ
// or Keys) if available. If enumeration is not possible a short message is
// returned indicating that limitation.
func DumpEnv(env Env) string {
	if env == nil {
		env = &OsEnv{}
	}
	entries := make(map[string]string)

	// Special-case TestEnv to expose its map and dedicated HOME/USER fields.
	if te, ok := env.(*TestEnv); ok {
		if te.data != nil {
			maps.Copy(entries, te.data)
		}
		if te.home != "" {
			entries["HOME"] = te.home
		}
		if te.user != "" {
			entries["USER"] = te.user
		}
	} else if _, ok := env.(*OsEnv); ok {
		// OsEnv: fall back to the process environment.
		for _, kv := range os.Environ() {
			if i := strings.Index(kv, "="); i >= 0 {
				entries[kv[:i]] = kv[i+1:]
			}
		}
	} else if en, ok := env.(interface{ Environ() []string }); ok {
		// Generic Environ() method returning "KEY=VAL" strings.
		for _, kv := range en.Environ() {
			if i := strings.Index(kv, "="); i >= 0 {
				entries[kv[:i]] = kv[i+1:]
			}
		}
	} else if ks, ok := env.(interface{ Keys() []string }); ok {
		// Generic Keys() method returning a list of keys.
		for _, k := range ks.Keys() {
			entries[k] = env.Get(k)
		}
	} else {
		return "env: cannot enumerate keys for this Env implementation"
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(entries[k])
		b.WriteByte('\n')
	}
	return b.String()
}
