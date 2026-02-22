package toolkit

import jailpkg "github.com/jlrickert/cli-toolkit/toolkit/jail"

// RemoveJailPrefix removes the jail prefix from a path and returns an absolute path.
func RemoveJailPrefix(jail, path string) string {
	return jailpkg.RemoveJailPrefix(jail, path)
}

// IsInJail reports whether the provided path resides within the jail boundary.
func IsInJail(jail, rel string) bool {
	return jailpkg.IsInJail(jail, rel)
}

// EnsureInJail returns a path that resides inside jail when possible.
func EnsureInJail(jail, p string) string {
	return jailpkg.EnsureInJail(jail, p)
}

// EnsureInJailFor is a test-friendly helper that mirrors EnsureInJail.
func EnsureInJailFor(jail, p string) string {
	return jailpkg.EnsureInJailFor(jail, p)
}
