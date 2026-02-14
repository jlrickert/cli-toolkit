package appctx_test

import (
	"path/filepath"
	"testing"

	proj "github.com/jlrickert/cli-toolkit/appctx"
	testutils "github.com/jlrickert/cli-toolkit/sandbox"
	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAppContextManualRootDefaults(t *testing.T) {
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
	p, err := proj.NewAppContext(f.Runtime(), manualRoot, appname)
	require.NoError(t, err)

	// Root should be exactly what we set.
	assert.Equal(t, manualRoot, p.Root)

	// Verify config/data/state/cache roots align with user-scoped paths joined
	// with the application name.
	ucfg, err := toolkit.UserConfigPath(f.Runtime().Env)
	require.NoError(t, err)
	expectedCfg := filepath.Join(ucfg, appname)
	assert.Equal(t, expectedCfg, p.ConfigRoot)

	udata, err := toolkit.UserDataPath(f.Runtime().Env)
	require.NoError(t, err)
	expectedData := filepath.Join(udata, appname)
	assert.Equal(t, expectedData, p.DataRoot)

	ustate, err := toolkit.UserStatePath(f.Runtime().Env)
	require.NoError(t, err)
	expectedState := filepath.Join(ustate, appname)
	assert.Equal(t, expectedState, p.StateRoot)

	ucache, err := toolkit.UserCachePath(f.Runtime().Env)
	require.NoError(t, err)
	expectedCache := filepath.Join(ucache, appname)
	assert.Equal(t, expectedCache, p.CacheRoot)
}
