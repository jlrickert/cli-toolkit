package toolkit_test

import (
	"testing"

	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMD5Hasher_DeterministicOutput(t *testing.T) {
	t.Parallel()

	h := &toolkit.MD5Hasher{}
	input := []byte("hello world")
	hash1 := h.Hash(input)
	hash2 := h.Hash(input)
	assert.Equal(t, hash1, hash2, "same input should produce same hash")
}

func TestMD5Hasher_DifferentInputs(t *testing.T) {
	t.Parallel()

	h := &toolkit.MD5Hasher{}
	hash1 := h.Hash([]byte("hello"))
	hash2 := h.Hash([]byte("world"))
	assert.NotEqual(t, hash1, hash2, "different inputs should produce different hashes")
}

func TestMD5Hasher_TrimsWhitespace(t *testing.T) {
	t.Parallel()

	h := &toolkit.MD5Hasher{}
	hash1 := h.Hash([]byte("content"))
	hash2 := h.Hash([]byte("  content  "))
	assert.Equal(t, hash1, hash2, "whitespace-trimmed inputs should produce same hash")
}

func TestMD5Hasher_EmptyInput(t *testing.T) {
	t.Parallel()

	h := &toolkit.MD5Hasher{}
	hash := h.Hash([]byte(""))
	assert.NotEmpty(t, hash, "empty input should still produce a hash")
	assert.Len(t, hash, 32, "MD5 hex digest should be 32 characters")
}

func TestMD5Hasher_HexFormat(t *testing.T) {
	t.Parallel()

	h := &toolkit.MD5Hasher{}
	hash := h.Hash([]byte("test"))
	// MD5 hex digest should be 32 lowercase hex characters.
	assert.Len(t, hash, 32)
	for _, c := range hash {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"hash should be lowercase hex, got char: %c", c)
	}
}

func TestOrDefaultHasher_WithHasher(t *testing.T) {
	t.Parallel()

	custom := &toolkit.MD5Hasher{}
	result := toolkit.OrDefaultHasher(custom)
	assert.Equal(t, custom, result)
}

func TestOrDefaultHasher_NilReturnsDefault(t *testing.T) {
	t.Parallel()

	result := toolkit.OrDefaultHasher(nil)
	require.NotNil(t, result)
	assert.Equal(t, toolkit.DefaultHasher, result)
}
