package toolkit_test

import (
	"runtime"
	"testing"

	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRuntimeWithEnv(t *testing.T, env toolkit.Env, jail string) *toolkit.Runtime {
	t.Helper()
	opts := []toolkit.RuntimeOption{
		toolkit.WithRuntimeEnv(env),
		toolkit.WithRuntimeFileSystem(&toolkit.OsFS{}),
	}
	if jail != "" {
		opts = append(opts, toolkit.WithRuntimeJail(jail))
	}
	rt, err := toolkit.NewRuntime(opts...)
	require.NoError(t, err)
	return rt
}

func TestAbsPath(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("skipping AbsPath tests on windows")
	}

	tests := []struct {
		name     string
		setup    func(*testing.T) *toolkit.Runtime
		input    string
		expected string
	}{
		{name: "empty path returns empty string", setup: func(t *testing.T) *toolkit.Runtime {
			return newRuntimeWithEnv(t, &toolkit.OsEnv{}, "")
		}, input: "", expected: ""},
		{name: "tilde alone expands to home", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/testuser", "testuser")
			return newRuntimeWithEnv(t, env, "")
		}, input: "~", expected: "/home/testuser"},
		{name: "tilde with path expands", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/testuser", "testuser")
			return newRuntimeWithEnv(t, env, "")
		}, input: "~/documents/file.txt", expected: "/home/testuser/documents/file.txt"},
		{name: "relative path converted to absolute", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			require.NoError(t, env.Setwd("/home/bob"))
			return newRuntimeWithEnv(t, env, "")
		}, input: "documents/file.txt", expected: "/home/bob/documents/file.txt"},
		{name: "relative path with dot", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			require.NoError(t, env.Setwd("/home/bob"))
			return newRuntimeWithEnv(t, env, "")
		}, input: "./config", expected: "/home/bob/config"},
		{name: "relative path with dot dot", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			require.NoError(t, env.Setwd("/home/bob/subdir"))
			return newRuntimeWithEnv(t, env, "")
		}, input: "../documents", expected: "/home/bob/documents"},
		{name: "absolute path unchanged", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: "/etc/passwd", expected: "/etc/passwd"},
		{name: "removes double slashes", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: "/home//bob//documents", expected: "/home/bob/documents"},
		{name: "removes trailing slash", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: "/home/bob/", expected: "/home/bob"},
		{name: "handles dot references", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: "/home/./bob/./documents", expected: "/home/bob/documents"},
		{name: "os env runtime", setup: func(t *testing.T) *toolkit.Runtime {
			return newRuntimeWithEnv(t, &toolkit.OsEnv{}, "")
		}, input: "/absolute/path", expected: "/absolute/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rt := tt.setup(t)
			result, err := rt.AbsPath(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolvePath(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("skipping ResolvePath tests on windows")
	}

	tests := []struct {
		name     string
		setup    func(*testing.T) *toolkit.Runtime
		input    string
		expected string
	}{
		{name: "empty path returns cwd", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/testuser", "testuser")
			return newRuntimeWithEnv(t, env, "")
		}, input: "", expected: "/home/testuser"},
		{name: "tilde alone expands to home", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/testuser", "testuser")
			return newRuntimeWithEnv(t, env, "")
		}, input: "~", expected: "/home/testuser"},
		{name: "tilde with path expands", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/testuser", "testuser")
			return newRuntimeWithEnv(t, env, "")
		}, input: "~/documents/file.txt", expected: "/home/testuser/documents/file.txt"},
		{name: "relative path converted to absolute", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			require.NoError(t, env.Setwd("/home/bob"))
			return newRuntimeWithEnv(t, env, "")
		}, input: "documents/file.txt", expected: "/home/bob/documents/file.txt"},
		{name: "relative path with dot", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			require.NoError(t, env.Setwd("/home/bob"))
			return newRuntimeWithEnv(t, env, "")
		}, input: "./config", expected: "/home/bob/config"},
		{name: "relative path with dot dot", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			require.NoError(t, env.Setwd("/home/bob/subdir"))
			return newRuntimeWithEnv(t, env, "")
		}, input: "../documents", expected: "/home/bob/documents"},
		{name: "absolute path unchanged", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: "/opt/homebrew/etc/passwd", expected: "/opt/homebrew/etc/passwd"},
		{name: "removes double slashes", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: "/home//bob//documents", expected: "/home/bob/documents"},
		{name: "removes trailing slash", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: "/home/bob/", expected: "/home/bob"},
		{name: "handles dot references", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", "/home/bob", "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: "/home/./bob/./documents", expected: "/home/bob/documents"},
		{name: "os env runtime", setup: func(t *testing.T) *toolkit.Runtime {
			return newRuntimeWithEnv(t, &toolkit.OsEnv{}, "")
		}, input: "/absolute/path", expected: "/absolute/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rt := tt.setup(t)
			result, err := rt.ResolvePath(tt.input, false)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRelativePath(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("skipping RelativePath tests on windows")
	}

	rt := newRuntimeWithEnv(t, &toolkit.OsEnv{}, "")

	tests := []struct {
		name     string
		basepath string
		path     string
		expected string
	}{
		{name: "empty path returns empty string", basepath: "/home/bob", path: "", expected: ""},
		{name: "same path returns dot", basepath: "/home/bob", path: "/home/bob", expected: "."},
		{name: "sibling directory", basepath: "/home/bob", path: "/home/alice", expected: "../alice"},
		{name: "child directory", basepath: "/home/bob", path: "/home/bob/documents", expected: "documents"},
		{name: "nested child directory", basepath: "/home/bob", path: "/home/bob/documents/work/file.txt", expected: "documents/work/file.txt"},
		{name: "parent directory", basepath: "/home/bob/documents", path: "/home/bob", expected: ".."},
		{name: "unrelated path", basepath: "/home/bob", path: "/var/log/system.log", expected: "../../var/log/system.log"},
		{name: "removes double slashes in result", basepath: "/home//bob", path: "/home/bob/documents", expected: "documents"},
		{name: "handles dot references in paths", basepath: "/home/./bob", path: "/home/bob/documents", expected: "documents"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := rt.RelativePath(tt.basepath, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
