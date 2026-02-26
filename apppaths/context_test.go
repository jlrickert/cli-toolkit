package appctx_test

import (
	"log/slog"
	"path/filepath"
	"testing"

	proj "github.com/jlrickert/cli-toolkit/apppaths"
	"github.com/jlrickert/cli-toolkit/mylog"
	testutils "github.com/jlrickert/cli-toolkit/sandbox"
	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAppPathsManualRootDefaults(t *testing.T) {
	t.Parallel()

	// Create fixture and populate with the example repo under "repo".
	f := NewSandbox(t,
		testutils.WithFixture("basic", "repo"),
		testutils.WithWd("repo/basic"),
	)

	// Build the project manually without using NewProject. Set the root to the
	// requested absolute path.
	appname := "myapp"
	manualRoot := filepath.FromSlash("/home/testuser/repo/basic")
	p, err := proj.NewAppPaths(f.Runtime(), manualRoot, appname)
	require.NoError(t, err)

	// Root should be exactly what we set.
	assert.Equal(t, manualRoot, p.Root)

	// Verify config/data/state/cache roots align with user-scoped paths joined
	// with the application name.
	ucfg, err := toolkit.UserConfigPath(f.Runtime())
	require.NoError(t, err)
	expectedCfg := filepath.Join(ucfg, appname)
	assert.Equal(t, expectedCfg, p.ConfigRoot)

	udata, err := toolkit.UserDataPath(f.Runtime())
	require.NoError(t, err)
	expectedData := filepath.Join(udata, appname)
	assert.Equal(t, expectedData, p.DataRoot)

	ustate, err := toolkit.UserStatePath(f.Runtime())
	require.NoError(t, err)
	expectedState := filepath.Join(ustate, appname)
	assert.Equal(t, expectedState, p.StateRoot)

	ucache, err := toolkit.UserCachePath(f.Runtime())
	require.NoError(t, err)
	expectedCache := filepath.Join(ucache, appname)
	assert.Equal(t, expectedCache, p.CacheRoot)
}

func TestFindGitRoot_NonGitDirectoryLogsDebugFallback(t *testing.T) {
	t.Parallel()

	f := NewSandbox(t,
		testutils.WithFixture("basic", "repo"),
		testutils.WithWd("/home/testuser"),
	)

	lg, th := mylog.NewTestLogger(t, slog.LevelDebug)
	require.NoError(t, f.Runtime().SetLogger(lg))

	root := proj.FindGitRoot(f.Context(), f.Runtime(), "/home/testuser")
	require.Equal(t, "", root)

	warns := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return e.Level == slog.LevelWarn && e.Msg == "git rev-parse failed, falling back"
	})
	require.Empty(t, warns, "non-git directories should not emit warn logs for fallback")
}
