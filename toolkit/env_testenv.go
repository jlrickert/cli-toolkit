package toolkit

import envpkg "github.com/jlrickert/cli-toolkit/toolkit/env"

// TestEnv is retained for backward compatibility.
type TestEnv = envpkg.TestEnv

// NewTestEnv is retained for backward compatibility.
func NewTestEnv(jail, home, username string) *TestEnv {
	return envpkg.NewTestEnv(jail, home, username)
}
