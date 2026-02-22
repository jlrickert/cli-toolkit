package toolkit

import (
	"bytes"
	"crypto/md5"
	"fmt"
)

// Hasher computes a deterministic short hash for a byte slice. Implementations
// should return a textual representation suitable for inclusion in meta fields.
type Hasher interface {
	Hash(data []byte) string
}

// MD5Hasher is a simple Hasher implementation that returns an MD5 hex digest.
//
// Note: MD5 is used here for deterministic, compact hashes only and is not
// intended for cryptographic integrity protection.
type MD5Hasher struct{}

// Hash implements Hasher by returning the lowercase hex MD5 of the trimmed
// input bytes.
func (m *MD5Hasher) Hash(data []byte) string {
	sum := md5.Sum(bytes.TrimSpace(data))
	return fmt.Sprintf("%x", sum[:])
}

// DefaultHasher is the fallback hasher used when none is provided.
var DefaultHasher Hasher = &MD5Hasher{}

// OrDefaultHasher returns h unless it is nil, in which case DefaultHasher is returned.
func OrDefaultHasher(h Hasher) Hasher {
	if h != nil {
		return h
	}
	return DefaultHasher
}

var _ Hasher = (*MD5Hasher)(nil)
