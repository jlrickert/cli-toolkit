package cfgcascade

import (
	"os"
	"strings"
)

// EnvProvider reads environment variables with a configurable prefix and
// returns a map[string]string of stripped, lowercased keys.
//
// For example, with prefix "TAP_" and env vars TAP_DEFAULT_KEG=home and
// TAP_LOG_LEVEL=debug, Load returns {"default_keg": "home", "log_level": "debug"}.
//
// If no matching env vars are found, Load returns os.ErrNotExist so the
// cascade treats it as an absent source.
type EnvProvider struct {
	// ProviderName is the human-readable name for this provider.
	ProviderName string

	// Prefix is the env var prefix to match (e.g. "TAP_"). Only variables
	// starting with this prefix are included.
	Prefix string

	// Keys lists the env var names (without prefix) to look up. If empty,
	// the provider returns ErrNotExist. Each key is looked up as Prefix+key.
	// For example, Keys=["DEFAULT_KEG", "LOG_LEVEL"] with Prefix="TAP_"
	// looks up TAP_DEFAULT_KEG and TAP_LOG_LEVEL.
	Keys []string
}

// Load reads environment variables matching Prefix+key for each key in Keys.
// The getenv parameter accepts a func(string) string for env var lookup,
// keeping cfgcascade decoupled from any specific env abstraction. Runtime-
// managed applications should pass their sandboxed lookup (e.g.
// rt.Env().Get) to preserve test isolation. If getenv is nil, falls back
// to os.Getenv for standalone use outside Runtime-managed applications.
func (p *EnvProvider) Load(getenv func(string) string) (map[string]string, error) {
	if getenv == nil {
		getenv = os.Getenv
	}

	result := make(map[string]string)
	for _, key := range p.Keys {
		envKey := p.Prefix + key
		val := getenv(envKey)
		if val != "" {
			stripped := strings.ToLower(key)
			result[stripped] = val
		}
	}

	if len(result) == 0 {
		return nil, os.ErrNotExist
	}
	return result, nil
}

func (p *EnvProvider) Name() string {
	return p.ProviderName
}
