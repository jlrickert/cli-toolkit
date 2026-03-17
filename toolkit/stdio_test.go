package toolkit_test

import (
	"bytes"
	"testing"

	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultStream(t *testing.T) {
	t.Parallel()

	s := toolkit.DefaultStream()
	require.NotNil(t, s)
	assert.NotNil(t, s.In, "stdin should not be nil")
	assert.NotNil(t, s.Out, "stdout should not be nil")
	assert.NotNil(t, s.Err, "stderr should not be nil")
}

func TestOrDefaultStream_WithStream(t *testing.T) {
	t.Parallel()

	custom := &toolkit.Stream{
		In:  &bytes.Buffer{},
		Out: &bytes.Buffer{},
		Err: &bytes.Buffer{},
	}

	result := toolkit.OrDefaultStream(custom)
	assert.Equal(t, custom, result)
}

func TestOrDefaultStream_NilReturnsDefault(t *testing.T) {
	t.Parallel()

	result := toolkit.OrDefaultStream(nil)
	require.NotNil(t, result)
	assert.NotNil(t, result.In)
	assert.NotNil(t, result.Out)
	assert.NotNil(t, result.Err)
}
