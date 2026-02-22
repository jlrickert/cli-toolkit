package toolkit

import envpkg "github.com/jlrickert/cli-toolkit/toolkit/env"

// Env is retained for backward compatibility. New code can import toolkit/env directly.
type Env = envpkg.Env

// EnvCloner is retained for backward compatibility.
type EnvCloner = envpkg.EnvCloner

// GetDefault returns the value of key from env when present and non-empty.
func GetDefault(env Env, key, other string) string {
	return envpkg.GetDefault(env, key, other)
}

// ExpandEnv expands $var or ${var} in s using env.
func ExpandEnv(env Env, s string) string {
	return envpkg.ExpandEnv(env, s)
}

// DumpEnv returns a sorted, newline separated representation of env.
func DumpEnv(env Env) string {
	return envpkg.DumpEnv(env)
}
