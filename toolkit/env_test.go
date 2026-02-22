package toolkit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type envOnly struct{}

func (envOnly) Name() string                   { return "env-only" }
func (envOnly) Get(string) string              { return "" }
func (envOnly) Set(string, string) error       { return nil }
func (envOnly) Has(string) bool                { return false }
func (envOnly) Environ() []string              { return nil }
func (envOnly) Unset(string)                   {}
func (envOnly) GetHome() (string, error)       { return "", nil }
func (envOnly) SetHome(string) error           { return nil }
func (envOnly) GetUser() (string, error)       { return "", nil }
func (envOnly) SetUser(string) error           { return nil }
func (envOnly) GetJail() string                { return "" }
func (envOnly) SetJail(string) error           { return nil }
func (envOnly) Getwd() (string, error)         { return "/", nil }
func (envOnly) Setwd(string) error             { return nil }
func (envOnly) GetTempDir() string             { return "/tmp" }
func (envOnly) CloneEnv() toolkit.Env          { return envOnly{} }
func (envOnly) String() string                 { return "env-only" }
func (envOnly) MarshalText() ([]byte, error)   { return []byte("env-only"), nil }
func (envOnly) UnmarshalText([]byte) error     { return nil }
func (envOnly) MarshalBinary() ([]byte, error) { return []byte("env-only"), nil }
func (envOnly) UnmarshalBinary([]byte) error   { return nil }

var _ toolkit.Env = (*envOnly)(nil)

func TestGetDefault(t *testing.T) {
	jail := t.TempDir()
	env := toolkit.NewTestEnv(jail, "", "")
	require.NoError(t, env.Set("EXIST", "val"))

	assert.Equal(t, "val", toolkit.GetDefault(env, "EXIST", "other"))

	require.NoError(t, env.Set("EMPTY", ""))
	assert.Equal(t, "def", toolkit.GetDefault(env, "EMPTY", "def"))

	assert.Equal(t, "fallback", toolkit.GetDefault(env, "MISSING", "fallback"))
}

func TestTestEnvDoesNotChangeOsEnv(t *testing.T) {
	const key = "GO_STD_TEST_OS_ENV_KEY"

	orig, ok := os.LookupEnv(key)
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, orig)
		} else {
			_ = os.Unsetenv(key)
		}
	})

	require.NoError(t, os.Setenv(key, "os-value"))

	jail := t.TempDir()
	env := toolkit.NewTestEnv(jail, "", "")
	require.NoError(t, env.Set(key, "test-value"))

	assert.Equal(t, "os-value", os.Getenv(key))

	env.Unset(key)
	assert.Equal(t, "os-value", os.Getenv(key))
}

func TestExpandEnv(t *testing.T) {
	jail := t.TempDir()
	env := toolkit.NewTestEnv(jail, "", "")
	require.NoError(t, env.Set("FOO", "bar"))
	require.NoError(t, env.Set("EMPTY", ""))

	got := toolkit.ExpandEnv(env, "$FOO/baz")
	assert.Equal(t, filepath.Join("bar", "baz"), got)

	got2 := toolkit.ExpandEnv(env, "${FOO}_${MISSING}_${EMPTY}")
	assert.Equal(t, "bar__", got2)

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

	got3 := toolkit.ExpandEnv(nil, "$"+oskey)
	assert.Equal(t, "osval", got3)
}

func TestTestEnvGlob(t *testing.T) {
	jail := t.TempDir()
	env := toolkit.NewTestEnv(jail, "", "")
	rt, err := toolkit.NewRuntime(
		toolkit.WithRuntimeEnv(env),
		toolkit.WithRuntimeFileSystem(&toolkit.OsFS{}),
		toolkit.WithRuntimeJail(jail),
	)
	require.NoError(t, err)

	require.NoError(t, rt.WriteFile("file1.txt", []byte("content1"), 0o644))
	require.NoError(t, rt.WriteFile("file2.txt", []byte("content2"), 0o644))
	require.NoError(t, rt.WriteFile("file3.md", []byte("content3"), 0o644))
	require.NoError(t, rt.Mkdir("subdir", 0o755, true))
	require.NoError(t, rt.WriteFile("subdir/file4.txt", []byte("content4"), 0o644))

	matches, err := rt.Glob("*.txt")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"file1.txt", "file2.txt"}, matches)

	matches, err = rt.Glob("subdir/*.txt")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"subdir/file4.txt"}, matches)

	matches, err = rt.Glob("*.nonexistent")
	require.NoError(t, err)
	assert.Empty(t, matches)

	matches, err = rt.Glob("*.*")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"file1.txt", "file2.txt", "file3.md"}, matches)
}

func TestOsEnvGlob(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file3.md"), []byte("content3"), 0o644))

	rt, err := toolkit.NewRuntime(
		toolkit.WithRuntimeEnv(&toolkit.OsEnv{}),
		toolkit.WithRuntimeFileSystem(&toolkit.OsFS{}),
	)
	require.NoError(t, err)

	pattern := filepath.Join(tmpDir, "*.txt")
	matches, err := rt.Glob(pattern)
	require.NoError(t, err)

	var expectedFiles []string
	for _, f := range []string{"file1.txt", "file2.txt"} {
		expectedFiles = append(expectedFiles, filepath.Join(tmpDir, f))
	}
	assert.ElementsMatch(t, expectedFiles, matches)
}
