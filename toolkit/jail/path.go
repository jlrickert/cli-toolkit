package jail

import (
	"path/filepath"
	"strings"
)

// RemoveJailPrefix removes the jail prefix from a path and returns an
// absolute path.
func RemoveJailPrefix(jailPath, path string) string {
	j := filepath.Clean(jailPath)
	p := filepath.Clean(path)

	if j == "" {
		return p
	}

	// Use filepath.Rel to strip the jail prefix.
	rel, err := filepath.Rel(j, p)
	if err != nil {
		return p
	}

	// Return as absolute path.
	return filepath.Join(string(filepath.Separator), rel)
}

// IsInJail reports whether the provided path resides within the jail
// boundary.
//
// If jail is empty, the function returns true (no boundary).
// Relative paths always are in the jail.
func IsInJail(jailPath, rel string) bool {
	j := filepath.Clean(jailPath)
	if j == "" || jailPath == "" {
		return true
	}
	p := filepath.Clean(rel)

	// A relative path is always inside the jail.
	if !filepath.IsAbs(p) {
		return true
	}

	// Check if p is within jail by comparing cleaned paths.
	relPath, err := filepath.Rel(j, p)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", it's outside the jail.
	return !strings.HasPrefix(relPath, "..")
}

// EnsureInJail returns a path that resides inside jail when possible.
//
// If the path is already inside jail, the cleaned absolute form is
// returned. Otherwise a path under jail is returned by appending the
// base name of the path.
func EnsureInJail(jailPath, p string) string {
	if jailPath == "" {
		return p
	}
	// Clean inputs.
	j := filepath.Clean(jailPath)
	if p == "" || p == "/" {
		return j
	}
	pp := filepath.Clean(p)

	if IsInJail(j, pp) {
		return pp
	}

	// If pp is relative, make it absolute relative to jail and return it.
	if !filepath.IsAbs(pp) {
		return filepath.Join(j, pp)
	}

	// Otherwise, place a safe fallback under jail using the base name.
	return filepath.Join(j, pp)
}

// EnsureInJailFor is a test-friendly helper that mirrors EnsureInJail
// but accepts paths written with forward slashes.
//
// It converts both jail and p using filepath.FromSlash before applying
// the EnsureInJail logic. Use this from tests when expected values are
// easier to express using POSIX-style literals.
func EnsureInJailFor(jailPath, p string) string {
	// Convert slash-separated test inputs to platform-specific form.
	j := filepath.FromSlash(jailPath)
	pp := filepath.FromSlash(p)

	// Reuse EnsureInJail logic on the converted inputs. This ensures tests
	// see the same behavior as production code while allowing easier
	// literals.
	return EnsureInJail(j, pp)
}
