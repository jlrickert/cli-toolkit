package toolkit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefault(t *testing.T) {
	jail := t.TempDir()
	env := toolkit.NewTestEnv(jail, "", "")
	require.NoError(t, env.Set("EXIST", "val"))

	assert.Equal(t, "val", toolkit.GetDefault(env, "EXIST", "other"))

	// empty value should fall back to provided default
	require.NoError(t, env.Set("EMPTY", ""))
	assert.Equal(t, "def", toolkit.GetDefault(env, "EMPTY", "def"))

	// missing key should return fallback
	assert.Equal(t, "fallback", toolkit.GetDefault(env, "MISSING", "fallback"))
}

// Ensure changing the test MapEnv does not modify the real process environment.
func TestTestEnvDoesNotChangeOsEnv(t *testing.T) {
	const key = "GO_STD_TEST_OS_ENV_KEY"

	// Preserve original OS env value and restore on exit.
	orig, ok := os.LookupEnv(key)
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, orig)
		} else {
			_ = os.Unsetenv(key)
		}
	})

	// Set a known value in the real OS environment.
	require.NoError(t, os.Setenv(key, "os-value"))

	// Create a test env and change the same key in the MapEnv.
	jail := t.TempDir()
	env := toolkit.NewTestEnv(jail, "", "")
	require.NoError(t, env.Set(key, "test-value"))

	// The real OS environment should remain unchanged.
	assert.Equal(t, "os-value", os.Getenv(key))

	// Unsetting in the MapEnv should not affect the real OS env either.
	env.Unset(key)
	assert.Equal(t, "os-value", os.Getenv(key))
}

func TestExpandEnv(t *testing.T) {
	jail := t.TempDir()
	// Do not run this test in parallel because it temporarily sets real OS env.
	env := toolkit.NewTestEnv(jail, "", "")
	require.NoError(t, env.Set("FOO", "bar"))
	require.NoError(t, env.Set("EMPTY", ""))

	ctx := toolkit.WithEnv(context.Background(), env)

	// Simple $VAR expansion
	got := toolkit.ExpandEnv(ctx, "$FOO/baz")
	assert.Equal(t, filepath.Join("bar", "baz"), got)

	// Braced form, missing and empty values
	got2 := toolkit.ExpandEnv(ctx, "${FOO}_${MISSING}_${EMPTY}")
	assert.Equal(t, "bar__", got2)

	// When no Env is provided in the context, ExpandEnv should fall back to the
	// real OS environment (OsEnv). Verify by setting a real OS env var.
	const oskey = "GO_STD_TEST_EXPAND_ENV_OS"
	orig, ok := os.LookupEnv(oskey)
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(oskey, orig)
		} else {
			_ = os.Unsetenv(oskey)
		}
	})
	require.NoError(t, os.Setenv(oskey, "osval"))

	got3 := toolkit.ExpandEnv(context.Background(), "$"+oskey)
	assert.Equal(t, "osval", got3)
}

func TestTestEnvGlob(t *testing.T) {
	jail := t.TempDir()
	env := toolkit.NewTestEnv(jail, "", "")
	ctx := toolkit.WithEnv(context.Background(), env)

	// Create test files in the jailed environment
	require.NoError(t, toolkit.WriteFile(ctx, "file1.txt", []byte("content1"), 0644))
	require.NoError(t, toolkit.WriteFile(ctx, "file2.txt", []byte("content2"), 0644))
	require.NoError(t, toolkit.WriteFile(ctx, "file3.md", []byte("content3"), 0644))
	require.NoError(t, toolkit.Mkdir(ctx, "subdir", 0755, true))
	require.NoError(t, toolkit.WriteFile(ctx, "subdir/file4.txt", []byte("content4"), 0644))

	// Test glob with *.txt pattern
	matches, err := toolkit.Glob(ctx, "*.txt")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"file1.txt", "file2.txt"}, matches)

	// Test glob with */*.txt pattern
	matches, err = toolkit.Glob(ctx, "subdir/*.txt")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"subdir/file4.txt"}, matches)

	// Test glob with no matches
	matches, err = toolkit.Glob(ctx, "*.nonexistent")
	require.NoError(t, err)
	assert.Empty(t, matches)

	// Test glob with multiple wildcards
	matches, err = toolkit.Glob(ctx, "*.*")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"file1.txt", "file2.txt", "file3.md"}, matches)
}

func TestOsEnvGlob(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file3.md"), []byte("content3"), 0644))

	env := &toolkit.OsEnv{}
	ctx := toolkit.WithEnv(context.Background(), env)

	// Test glob with absolute path
	pattern := filepath.Join(tmpDir, "*.txt")
	matches, err := toolkit.Glob(ctx, pattern)
	require.NoError(t, err)
	// Normalize paths for comparison
	var expectedFiles []string
	for _, f := range []string{"file1.txt", "file2.txt"} {
		expectedFiles = append(expectedFiles, filepath.Join(tmpDir, f))
	}
	assert.ElementsMatch(t, expectedFiles, matches)
}
