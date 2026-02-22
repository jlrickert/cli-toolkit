package toolkit

import jailpkg "github.com/jlrickert/cli-toolkit/toolkit/jail"

// Jailed is retained for backward compatibility. New code can import
// toolkit/jail directly.
type Jailed = jailpkg.Jailed

// IsJailed reports whether j has a non-empty jail configured.
func IsJailed(j Jailed) bool {
	return jailpkg.IsJailed(j)
}
