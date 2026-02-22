package sandbox_test

import (
	"path/filepath"
	"testing"

	tu "github.com/jlrickert/cli-toolkit/sandbox"
	"github.com/stretchr/testify/require"
)

func TestSandbox_BasicSetup(t *testing.T) {
	t.Parallel()

	sandbox := tu.NewSandbox(t, nil)

	ctx := sandbox.Context()
	require.NotNil(t, ctx)
	require.NotNil(t, sandbox.Runtime())
	require.NotNil(t, sandbox.Runtime().Env())
	require.NotNil(t, sandbox.Runtime().FS())
	require.NotNil(t, sandbox.Runtime().Logger())
	require.NotNil(t, sandbox.Runtime().Clock())
}

func TestSandbox_WithFixture(t *testing.T) {
	t.Parallel()

	sandbox := tu.NewSandbox(t, &tu.Options{Data: testdata}, tu.WithFixture("example", "~/fixtures/example"))

	data := sandbox.MustReadFile("fixtures/example/example.txt")
	require.NotEmpty(t, data)
}

func TestSandbox_RuntimeCarriesStream(t *testing.T) {
	t.Parallel()

	sandbox := tu.NewSandbox(t, nil)

	stream := sandbox.Runtime().Stream()
	require.NotNil(t, stream)
	require.NotNil(t, stream.In)
	require.NotNil(t, stream.Out)
	require.NotNil(t, stream.Err)
}

func TestSandbox_RuntimeCarriesHasher(t *testing.T) {
	t.Parallel()

	sandbox := tu.NewSandbox(t, nil)

	hasher := sandbox.Runtime().Hasher()
	require.NotNil(t, hasher)
	require.NotEmpty(t, hasher.Hash([]byte("test")))
}

func TestSandbox_RuntimeCarriesClock(t *testing.T) {
	t.Parallel()

	sandbox := tu.NewSandbox(t, nil)

	clk := sandbox.Runtime().Clock()
	require.NotNil(t, clk)

	now := clk.Now()
	require.False(t, now.IsZero())
}

func TestSandbox_RuntimeCarriesEnv(t *testing.T) {
	t.Parallel()

	sandbox := tu.NewSandbox(t, nil)
	env := sandbox.Runtime().Env()
	require.NotNil(t, env)

	home, err := env.GetHome()
	require.NoError(t, err)
	require.NotEmpty(t, home)
}

func TestSandbox_RuntimeCarriesLogger(t *testing.T) {
	t.Parallel()

	sandbox := tu.NewSandbox(t, nil)

	logger := sandbox.Runtime().Logger()
	require.NotNil(t, logger)
}

func TestSandbox_MultipleContexts(t *testing.T) {
	t.Parallel()

	sandbox1 := tu.NewSandbox(t, nil, tu.WithEnv("TEST_KEY", "value1"))
	sandbox2 := tu.NewSandbox(t, nil, tu.WithEnv("TEST_KEY", "value2"))

	env1 := sandbox1.Runtime().Env()
	env2 := sandbox2.Runtime().Env()

	require.Equal(t, "value1", env1.Get("TEST_KEY"))
	require.Equal(t, "value2", env2.Get("TEST_KEY"))
}

func TestSandbox_RuntimePersistsAcrossOperations(t *testing.T) {
	t.Parallel()

	sandbox := tu.NewSandbox(t, nil)
	env := sandbox.Runtime().Env()
	err := env.Set("PERSIST_KEY", "persist_value")
	require.NoError(t, err)

	env2 := sandbox.Runtime().Env()
	require.Equal(t, "persist_value", env2.Get("PERSIST_KEY"))
}

func TestSandbox_ContextWithCustomOptions(t *testing.T) {
	t.Parallel()

	sandbox := tu.NewSandbox(t, nil,
		tu.WithEnv("CUSTOM_VAR", "custom_value"),
		tu.WithEnv("DEBUG", "true"),
	)

	env := sandbox.Runtime().Env()
	require.Equal(t, "custom_value", env.Get("CUSTOM_VAR"))
	require.Equal(t, "true", env.Get("DEBUG"))
}

func TestSandbox_ResolvePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
		cwd      string
	}{
		{name: "relative path", input: "test.txt", expected: filepath.Join("/", "home", "testuser", "test.txt")},
		{name: "tilde expansion", input: "~/test.txt", expected: filepath.Join("/", "home", "testuser", "test.txt")},
		{name: "escape attempt with dot dot", input: "../../../escape.txt", expected: filepath.Join("/escape.txt")},
		{name: "respects working directory", cwd: filepath.Join("~", ".config", "app"), input: "../../repos/GitHub.com", expected: filepath.Join("/", "home", "testuser", "repos", "GitHub.com")},
		{name: "absolute path", input: "/opt/etc/passwd", expected: filepath.Join("/", "opt", "etc", "passwd")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sandbox := tu.NewSandbox(t, nil)

			if tc.cwd != "" {
				require.NoError(t, sandbox.Setwd(tc.cwd))
			}
			resolved, err := sandbox.ResolvePath(tc.input)
			require.NoError(t, err)
			require.NotEmpty(t, resolved)

			cwd, err := sandbox.Getwd()
			require.NoError(t, err)
			require.Equal(t, tc.expected, resolved, "cwd is %s", cwd)
		})
	}
}
