package jail

import "strings"

// Jailed describes components that can report and update a jail root.
//
// Implementations should treat an empty jail as "not jailed".
type Jailed interface {
	// GetJail returns the current jail root.
	GetJail() string
	// SetJail updates the current jail root.
	SetJail(jail string) error
}

// IsJailed reports whether j has a non-empty jail configured.
func IsJailed(j Jailed) bool {
	if j == nil {
		return false
	}
	return strings.TrimSpace(j.GetJail()) != ""
}
