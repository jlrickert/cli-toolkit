package toolkit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntime_ReadWriteFile(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	data := []byte("hello world")
	require.NoError(t, rt.WriteFile("test.txt", data, 0o644))

	got, err := rt.ReadFile("test.txt")
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestRuntime_Mkdir(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	require.NoError(t, rt.Mkdir("newdir/sub", 0o755, true))

	info, err := rt.Stat("newdir/sub", false)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestRuntime_Remove(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	require.NoError(t, rt.WriteFile("todelete.txt", []byte("bye"), 0o644))

	_, err = rt.Stat("todelete.txt", false)
	require.NoError(t, err)

	require.NoError(t, rt.Remove("todelete.txt", false))

	_, err = rt.Stat("todelete.txt", false)
	assert.True(t, os.IsNotExist(err))
}

func TestRuntime_RemoveAll(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	require.NoError(t, rt.Mkdir("removeme/sub", 0o755, true))
	require.NoError(t, rt.WriteFile("removeme/sub/file.txt", []byte("data"), 0o644))

	require.NoError(t, rt.Remove("removeme", true))

	_, err = rt.Stat("removeme", false)
	assert.True(t, os.IsNotExist(err))
}

func TestRuntime_Rename(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	require.NoError(t, rt.WriteFile("old.txt", []byte("content"), 0o644))
	require.NoError(t, rt.Rename("old.txt", "new.txt"))

	_, err = rt.Stat("old.txt", false)
	assert.True(t, os.IsNotExist(err))

	got, err := rt.ReadFile("new.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("content"), got)
}

func TestRuntime_Stat(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	require.NoError(t, rt.WriteFile("stat.txt", []byte("data"), 0o644))

	info, err := rt.Stat("stat.txt", false)
	require.NoError(t, err)
	assert.Equal(t, "stat.txt", info.Name())
	assert.False(t, info.IsDir())
}

func TestRuntime_ReadDir(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	// Use WriteFile which auto-creates parent directories.
	require.NoError(t, rt.WriteFile("mydir/a.txt", []byte("a"), 0o644))
	require.NoError(t, rt.WriteFile("mydir/b.txt", []byte("b"), 0o644))

	entries, err := rt.ReadDir("mydir")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestRuntime_AtomicWriteFile(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	data := []byte("atomic data")
	require.NoError(t, rt.AtomicWriteFile("atomic.txt", data, 0o644))

	got, err := rt.ReadFile("atomic.txt")
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestRuntime_Rel(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	require.NoError(t, rt.Mkdir("a/b", 0o755, true))
	require.NoError(t, rt.Mkdir("a/c", 0o755, true))

	rel, err := rt.Rel("a/b", "a/c/file.txt")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("..", "c", "file.txt"), rel)
}

func TestRuntime_Setwd(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	// Create the directory first using all=true to ensure parents exist.
	require.NoError(t, rt.Mkdir("work", 0o755, true))
	require.NoError(t, rt.Setwd("work"))

	wd, err := rt.Getwd()
	require.NoError(t, err)
	assert.Contains(t, wd, "work")
}

func TestRuntime_Clone(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	clone := rt.Clone()
	require.NotNil(t, clone)

	// Modifying clone should not affect original.
	require.NoError(t, clone.Set("CLONE_VAR", "value"))
	assert.Empty(t, rt.Get("CLONE_VAR"))
}

func TestRuntime_Validate_NilRuntime(t *testing.T) {
	t.Parallel()

	var rt *toolkit.Runtime
	err := rt.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestRuntime_Symlink(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	require.NoError(t, rt.WriteFile("target.txt", []byte("target"), 0o644))

	err = rt.Symlink("target.txt", "link.txt")
	if err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	// Read through the link.
	got, err := rt.ReadFile("link.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("target"), got)
}

func TestRuntime_EnvForwarding(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	assert.Equal(t, "test-env", rt.Name())

	require.NoError(t, rt.Set("MY_VAR", "hello"))
	assert.Equal(t, "hello", rt.Get("MY_VAR"))
	assert.True(t, rt.Has("MY_VAR"))

	environ := rt.Environ()
	assert.NotEmpty(t, environ)

	rt.Unset("MY_VAR")
	assert.Empty(t, rt.Get("MY_VAR"))

	home, err := rt.GetHome()
	require.NoError(t, err)
	assert.NotEmpty(t, home)

	user, err := rt.GetUser()
	require.NoError(t, err)
	assert.Equal(t, "testuser", user)
}

func TestNewRuntime_NilOptionSkipped(t *testing.T) {
	t.Parallel()

	// Passing nil options should not cause a panic.
	rt, err := toolkit.NewRuntime(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, rt)
}

func TestRuntime_HostPath_TildeExpansion(t *testing.T) {
	t.Parallel()

	jailDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	rt, err := toolkit.NewTestRuntime(jailDir, "/home/alice", "alice")
	require.NoError(t, err)

	got, err := rt.HostPath("~/notes/keg")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(jailDir, "home", "alice", "notes", "keg"), got)
}

func TestRuntime_HostPath_EnvVarExpansion(t *testing.T) {
	t.Parallel()

	jailDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	rt, err := toolkit.NewTestRuntime(jailDir, "/home/alice", "alice")
	require.NoError(t, err)
	require.NoError(t, rt.Set("KEG_ROOT", "/home/alice/notes"))

	got, err := rt.HostPath("$KEG_ROOT/keg")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(jailDir, "home", "alice", "notes", "keg"), got)
}

func TestRuntime_HostPath_AbsoluteVirtualPath(t *testing.T) {
	t.Parallel()

	jailDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	rt, err := toolkit.NewTestRuntime(jailDir, "/home/alice", "alice")
	require.NoError(t, err)

	got, err := rt.HostPath("/etc/config")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(jailDir, "etc", "config"), got)
}

func TestRuntime_HostPath_NoJail(t *testing.T) {
	t.Parallel()

	// No jail: HostPath should return the cleaned absolute host path.
	cwd, err := os.Getwd()
	require.NoError(t, err)

	rt, err := toolkit.NewRuntime(
		toolkit.WithRuntimeFileSystem(&toolkit.OsFS{}),
	)
	require.NoError(t, err)

	abs := filepath.Join(cwd, "some", "file.txt")
	got, err := rt.HostPath(abs)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(abs), got)
}
