package toolkit_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserConfigPath(t *testing.T) {
	env := toolkit.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")
	require.NoError(t, env.Set("XDG_CONFIG_HOME", "/real/xdg"))
	cfg, err := toolkit.UserConfigPath(env)
	require.NoError(t, err)
	assert.Equal(t, "/real/xdg", cfg)

	env.Unset("XDG_CONFIG_HOME")

	if runtime.GOOS == "windows" {
		env := toolkit.NewTestEnv("", filepath.FromSlash("C:/Users/alice"), "alice")
		require.NoError(t, env.Set("APPDATA", filepath.FromSlash("C:/AppData")))
		cfg, err := toolkit.UserConfigPath(env)
		require.NoError(t, err)
		assert.Equal(t, filepath.FromSlash("C:/AppData"), cfg)
	} else {
		env := toolkit.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")
		cfg, err := toolkit.UserConfigPath(env)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join("/home/alice", ".config"), cfg)

		emptyEnv := &toolkit.TestEnv{}
		_, err = toolkit.UserConfigPath(emptyEnv)
		require.Error(t, err)
	}
}

func TestExpandPath(t *testing.T) {
	t.Parallel()

	t.Run("TildeOnly", func(t *testing.T) {
		jail := t.TempDir()
		relHome := filepath.Join("home", "alice")
		env := toolkit.NewTestEnv(jail, relHome, "alice")
		got, err := toolkit.ExpandPath(env, "~")
		require.NoError(t, err)

		expected := toolkit.EnsureInJail(jail, relHome)
		assert.Equal(t, filepath.Clean(expected), filepath.Clean(got))
	})

	t.Run("TildeSlash", func(t *testing.T) {
		jail := t.TempDir()
		env := toolkit.NewTestEnv(jail, filepath.Join("home", "alice"), "alice")
		got, err := toolkit.ExpandPath(env, filepath.Join("~", "project"))
		require.NoError(t, err)

		expected := toolkit.EnsureInJail(jail, filepath.Join("home", "alice", "project"))
		assert.Equal(t, filepath.Clean(expected), filepath.Clean(got))
	})

	t.Run("NonTildeUnchanged", func(t *testing.T) {
		env := toolkit.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")
		in := filepath.FromSlash("/tmp/some/path")
		got, err := toolkit.ExpandPath(env, in)
		require.NoError(t, err)
		assert.Equal(t, in, got)
	})

	t.Run("EmptyString", func(t *testing.T) {
		env := toolkit.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")
		got, err := toolkit.ExpandPath(env, "")
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})

	t.Run("UnsupportedUserFormReturnsUnchanged", func(t *testing.T) {
		env := toolkit.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")
		in := "~bob/project"
		got, err := toolkit.ExpandPath(env, in)
		require.NoError(t, err)
		assert.Equal(t, in, got)
	})

	t.Run("MissingHomeReturnsError", func(t *testing.T) {
		emptyEnv := &toolkit.TestEnv{}
		_, err := toolkit.ExpandPath(emptyEnv, "~")
		require.Error(t, err)
	})

	if runtime.GOOS == "windows" {
		t.Run("TildeBackslashWindows", func(t *testing.T) {
			home := filepath.FromSlash("C:/Users/alice")
			env := toolkit.NewTestEnv("", home, "alice")
			in := `~\project\sub`
			got, err := toolkit.ExpandPath(env, in)
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(home, "project", "sub"), got)
		})
	}
}

func TestUserCachePath(t *testing.T) {
	env := toolkit.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")
	require.NoError(t, env.Set("XDG_CACHE_HOME", "/xdg/cache"))
	c, err := toolkit.UserCachePath(env)
	require.NoError(t, err)
	assert.Equal(t, "/xdg/cache", c)

	env = toolkit.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")
	require.NoError(t, env.Set("LOCALAPPDATA", filepath.FromSlash("C:/Local")))
	c, err = toolkit.UserCachePath(env)
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		assert.Equal(t, filepath.FromSlash("C:/Local"), c)
	} else {
		assert.Equal(t, filepath.Join("/home/alice", ".cache"), c)
	}

	env = toolkit.NewTestEnv("", filepath.FromSlash("/home/bob"), "bob")
	c, err = toolkit.UserCachePath(env)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/home/bob", ".cache"), c)

	emptyEnv := &toolkit.TestEnv{}
	_, err = toolkit.UserCachePath(emptyEnv)
	require.Error(t, err)
}

func TestUserDataPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		env := toolkit.NewTestEnv("", filepath.FromSlash("C:/Users/alice"), "alice")
		require.NoError(t, env.Set("LOCALAPPDATA", filepath.FromSlash("C:/LocalApp")))
		p, err := toolkit.UserDataPath(env)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(filepath.FromSlash("C:/LocalApp"), "data"), p)

		env2 := toolkit.NewTestEnv("", filepath.FromSlash("C:/Users/alice"), "alice")
		env2.Unset("LOCALAPPDATA")
		_, err = toolkit.UserDataPath(env2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), toolkit.ErrNoEnvKey.Error())
	} else {
		env := toolkit.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")
		require.NoError(t, env.Set("XDG_DATA_HOME", "/xdg/data"))
		p, err := toolkit.UserDataPath(env)
		require.NoError(t, err)
		assert.Equal(t, "/xdg/data", p)

		env2 := toolkit.NewTestEnv("", filepath.FromSlash("/home/bob"), "bob")
		env2.Unset("XDG_DATA_HOME")
		p, err = toolkit.UserDataPath(env2)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join("/home/bob", ".local", "share"), p)

		emptyEnv := &toolkit.TestEnv{}
		_, err = toolkit.UserDataPath(emptyEnv)
		require.Error(t, err)
	}
}

func TestUserStatePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		env := toolkit.NewTestEnv("", filepath.FromSlash("C:/Users/alice"), "alice")
		require.NoError(t, env.Set("LOCALAPPDATA", filepath.FromSlash("C:/LocalApp")))
		p, err := toolkit.UserStatePath(env)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(filepath.FromSlash("C:/LocalApp"), "state"), p)

		env2 := toolkit.NewTestEnv("", filepath.FromSlash("C:/Users/alice"), "alice")
		env2.Unset("LOCALAPPDATA")
		_, err = toolkit.UserStatePath(env2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), toolkit.ErrNoEnvKey.Error())
	} else {
		env := toolkit.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")
		require.NoError(t, env.Set("XDG_STATE_HOME", "/xdg/state"))
		p, err := toolkit.UserStatePath(env)
		require.NoError(t, err)
		assert.Equal(t, "/xdg/state", p)

		env2 := toolkit.NewTestEnv("", filepath.FromSlash("/home/bob"), "bob")
		env2.Unset("XDG_STATE_HOME")
		p, err = toolkit.UserStatePath(env2)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join("/home/bob", ".local", "state"), p)

		emptyEnv := &toolkit.TestEnv{}
		_, err = toolkit.UserStatePath(emptyEnv)
		require.Error(t, err)
	}
}

func TestEdit_UsesRuntimeAndResolvesSymlinkPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("editor command fixture uses /bin/sh")
	}

	jailDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	rt, err := toolkit.NewTestRuntime(jailDir, "/home/alice", "alice")
	require.NoError(t, err)

	require.NoError(t, rt.Mkdir("/home/alice", 0o755, true))
	require.NoError(t, rt.Mkdir("/home/alice/work", 0o755, true))

	targetPath := "/home/alice/work/keg"
	require.NoError(t, rt.WriteFile(targetPath, []byte("kegv: \"2025-07\"\n"), 0o644))
	require.NoError(t, rt.Symlink(targetPath, "/home/alice/keg-link"))

	capturePath := filepath.Join(jailDir, "captured-editor-arg.txt")
	scriptPath := filepath.Join(jailDir, "capture-editor.sh")
	script := "#!/bin/sh\nprintf '%s' \"$1\" > \"$CAPTURE_FILE\"\n"
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))

	t.Setenv("EDITOR", "/bin/false")
	require.NoError(t, rt.Set("CAPTURE_FILE", capturePath))
	require.NoError(t, rt.Set("EDITOR", "/bin/sh "+scriptPath))
	rt.Unset("VISUAL")

	err = toolkit.Edit(context.Background(), rt, "~/keg-link")
	require.NoError(t, err)

	raw, err := os.ReadFile(capturePath)
	require.NoError(t, err)
	got := strings.TrimSpace(string(raw))
	want := filepath.Join(jailDir, "home", "alice", "work", "keg")
	require.Equal(t, filepath.Clean(want), filepath.Clean(got))
}
