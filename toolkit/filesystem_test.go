package toolkit_test

import (
	"os"
	"path/filepath"
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

func rootedPath(parts ...string) string {
	if len(parts) == 0 {
		return string(filepath.Separator)
	}

	elems := make([]string, 0, len(parts)+1)
	elems = append(elems, string(filepath.Separator))
	elems = append(elems, parts...)
	return filepath.Clean(filepath.Join(elems...))
}

type pathFixture struct {
	root         string
	testUserHome string
	bobHome      string
	bobSubdir    string
	bobDocs      string
}

func newPathFixture(t *testing.T) pathFixture {
	t.Helper()

	root := t.TempDir()
	homeRoot := filepath.Join(root, "home")
	return pathFixture{
		root:         root,
		testUserHome: filepath.Join(homeRoot, "testuser"),
		bobHome:      filepath.Join(homeRoot, "bob"),
		bobSubdir:    filepath.Join(homeRoot, "bob", "subdir"),
		bobDocs:      filepath.Join(homeRoot, "bob", "documents"),
	}
}

func TestAbsPath(t *testing.T) {
	t.Parallel()

	paths := newPathFixture(t)
	sep := string(filepath.Separator)
	dottedDocs := filepath.Join(paths.root, "home") + sep + "." + sep + "bob" + sep + "." + sep + "documents"
	doubleSlashDocs := paths.bobHome + sep + sep + "documents"
	relativeConfig := "." + sep + "config"
	relativeParentDocs := ".." + sep + "documents"
	osEnvAbs := rootedPath("absolute", "path")

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
			env := toolkit.NewTestEnv("", paths.testUserHome, "testuser")
			return newRuntimeWithEnv(t, env, "")
		}, input: "~", expected: paths.testUserHome},
		{name: "tilde with path expands", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.testUserHome, "testuser")
			return newRuntimeWithEnv(t, env, "")
		}, input: filepath.Join("~", "documents", "file.txt"), expected: filepath.Join(paths.testUserHome, "documents", "file.txt")},
		{name: "relative path converted to absolute", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			require.NoError(t, env.Setwd(paths.bobHome))
			return newRuntimeWithEnv(t, env, "")
		}, input: filepath.Join("documents", "file.txt"), expected: filepath.Join(paths.bobHome, "documents", "file.txt")},
		{name: "relative path with dot", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			require.NoError(t, env.Setwd(paths.bobHome))
			return newRuntimeWithEnv(t, env, "")
		}, input: relativeConfig, expected: filepath.Join(paths.bobHome, "config")},
		{name: "relative path with dot dot", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			require.NoError(t, env.Setwd(paths.bobSubdir))
			return newRuntimeWithEnv(t, env, "")
		}, input: relativeParentDocs, expected: paths.bobDocs},
		{name: "absolute path unchanged", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: filepath.Join(paths.root, "etc", "passwd"), expected: filepath.Join(paths.root, "etc", "passwd")},
		{name: "removes double slashes", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: doubleSlashDocs, expected: paths.bobDocs},
		{name: "removes trailing slash", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: paths.bobHome + sep, expected: paths.bobHome},
		{name: "handles dot references", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: dottedDocs, expected: paths.bobDocs},
		{name: "os env runtime", setup: func(t *testing.T) *toolkit.Runtime {
			return newRuntimeWithEnv(t, &toolkit.OsEnv{}, "")
		}, input: osEnvAbs, expected: osEnvAbs},
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

	paths := newPathFixture(t)
	sep := string(filepath.Separator)
	dottedDocs := filepath.Join(paths.root, "home") + sep + "." + sep + "bob" + sep + "." + sep + "documents"
	doubleSlashDocs := paths.bobHome + sep + sep + "documents"
	relativeConfig := "." + sep + "config"
	relativeParentDocs := ".." + sep + "documents"
	osEnvAbs := rootedPath("absolute", "path")

	tests := []struct {
		name     string
		setup    func(*testing.T) *toolkit.Runtime
		input    string
		expected string
	}{
		{name: "empty path returns cwd", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.testUserHome, "testuser")
			return newRuntimeWithEnv(t, env, "")
		}, input: "", expected: paths.testUserHome},
		{name: "tilde alone expands to home", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.testUserHome, "testuser")
			return newRuntimeWithEnv(t, env, "")
		}, input: "~", expected: paths.testUserHome},
		{name: "tilde with path expands", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.testUserHome, "testuser")
			return newRuntimeWithEnv(t, env, "")
		}, input: filepath.Join("~", "documents", "file.txt"), expected: filepath.Join(paths.testUserHome, "documents", "file.txt")},
		{name: "relative path converted to absolute", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			require.NoError(t, env.Setwd(paths.bobHome))
			return newRuntimeWithEnv(t, env, "")
		}, input: filepath.Join("documents", "file.txt"), expected: filepath.Join(paths.bobHome, "documents", "file.txt")},
		{name: "relative path with dot", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			require.NoError(t, env.Setwd(paths.bobHome))
			return newRuntimeWithEnv(t, env, "")
		}, input: relativeConfig, expected: filepath.Join(paths.bobHome, "config")},
		{name: "relative path with dot dot", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			require.NoError(t, env.Setwd(paths.bobSubdir))
			return newRuntimeWithEnv(t, env, "")
		}, input: relativeParentDocs, expected: paths.bobDocs},
		{name: "absolute path unchanged", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: filepath.Join(paths.root, "opt", "homebrew", "etc", "passwd"), expected: filepath.Join(paths.root, "opt", "homebrew", "etc", "passwd")},
		{name: "removes double slashes", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: doubleSlashDocs, expected: paths.bobDocs},
		{name: "removes trailing slash", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: paths.bobHome + sep, expected: paths.bobHome},
		{name: "handles dot references", setup: func(t *testing.T) *toolkit.Runtime {
			env := toolkit.NewTestEnv("", paths.bobHome, "bob")
			return newRuntimeWithEnv(t, env, "")
		}, input: dottedDocs, expected: paths.bobDocs},
		{name: "os env runtime", setup: func(t *testing.T) *toolkit.Runtime {
			return newRuntimeWithEnv(t, &toolkit.OsEnv{}, "")
		}, input: osEnvAbs, expected: osEnvAbs},
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

func TestOsFS_WorkingDirectory_InternalState(t *testing.T) {
	t.Parallel()

	processWd, err := os.Getwd()
	require.NoError(t, err)

	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	require.NoError(t, os.MkdirAll(workDir, 0o755))

	fs := &toolkit.OsFS{}
	require.NoError(t, fs.Setwd(workDir))

	wd, err := fs.Getwd()
	require.NoError(t, err)
	require.Equal(t, filepath.Clean(workDir), wd)

	resolved, err := fs.ResolvePath("note.txt", false)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(workDir, "note.txt"), resolved)

	require.NoError(t, fs.WriteFile("note.txt", []byte("hello"), 0o644))
	data, err := os.ReadFile(filepath.Join(workDir, "note.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(data))

	afterProcessWd, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, processWd, afterProcessWd)
}

func TestOsFS_ResolvePath_FallbackToProcessWorkingDirectory(t *testing.T) {
	t.Parallel()

	processWd, err := os.Getwd()
	require.NoError(t, err)

	fs := &toolkit.OsFS{}
	resolved, err := fs.ResolvePath("file.txt", false)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(processWd, "file.txt"), resolved)
}

func TestOsFS_Rel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	require.NoError(t, os.MkdirAll(filepath.Join(workDir, "docs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workDir, "notes"), 0o755))

	fs := &toolkit.OsFS{}
	require.NoError(t, fs.Setwd(workDir))

	rel, err := fs.Rel("docs", filepath.Join("notes", "todo.txt"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join("..", "notes", "todo.txt"), rel)
}

func TestRelativePath(t *testing.T) {
	t.Parallel()

	paths := newPathFixture(t)

	rt := newRuntimeWithEnv(t, &toolkit.OsEnv{}, "")

	tests := []struct {
		name     string
		basepath string
		path     string
		expected string
	}{
		{name: "empty path returns empty string", basepath: paths.bobHome, path: "", expected: ""},
		{name: "same path returns dot", basepath: paths.bobHome, path: paths.bobHome, expected: "."},
		{name: "sibling directory", basepath: paths.bobHome, path: filepath.Join(paths.root, "home", "alice"), expected: filepath.Join("..", "alice")},
		{name: "child directory", basepath: paths.bobHome, path: paths.bobDocs, expected: "documents"},
		{name: "nested child directory", basepath: paths.bobHome, path: filepath.Join(paths.bobDocs, "work", "file.txt"), expected: filepath.Join("documents", "work", "file.txt")},
		{name: "parent directory", basepath: paths.bobDocs, path: paths.bobHome, expected: ".."},
		{name: "unrelated path", basepath: paths.bobHome, path: filepath.Join(paths.root, "var", "log", "system.log"), expected: filepath.Join("..", "..", "var", "log", "system.log")},
		{name: "removes double slashes in result", basepath: paths.bobHome + string(filepath.Separator) + string(filepath.Separator), path: paths.bobDocs, expected: "documents"},
		{name: "handles dot references in paths", basepath: filepath.Join(paths.root, "home") + string(filepath.Separator) + "." + string(filepath.Separator) + "bob", path: paths.bobDocs, expected: "documents"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := rt.RelativePath(tt.basepath, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOsFS_WorkingDirectoryAndJail(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(jail, "work"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(jail, "next"), 0o755))

	workVirtual := rootedPath("work")
	nextVirtual := rootedPath("next")
	workNoteVirtual := rootedPath("work", "note.txt")

	fs, err := toolkit.NewOsFS(jail, workVirtual)
	require.NoError(t, err)

	wd, err := fs.Getwd()
	require.NoError(t, err)
	require.Equal(t, workVirtual, wd)

	require.NoError(t, fs.WriteFile("note.txt", []byte("hello"), 0o644))

	data, err := os.ReadFile(filepath.Join(jail, "work", "note.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(data))

	require.NoError(t, fs.Setwd(nextVirtual))
	wd, err = fs.Getwd()
	require.NoError(t, err)
	require.Equal(t, nextVirtual, wd)

	resolved, err := fs.ResolvePath(filepath.Join("..", "work", "note.txt"), false)
	require.NoError(t, err)
	require.Equal(t, workNoteVirtual, resolved)

	got, err := fs.ReadFile(filepath.Join("..", "work", "note.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))
}

func TestOsFS_Rel_Jailed(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	workVirtual := rootedPath("work")
	fs, err := toolkit.NewOsFS(jail, workVirtual)
	require.NoError(t, err)

	rel, err := fs.Rel("docs", filepath.Join("notes", "todo.txt"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join("..", "notes", "todo.txt"), rel)
}

func TestOsFS_Glob_RelativeAndAbsolute_Jailed(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(jail, "glob"), 0o755))

	globVirtual := rootedPath("glob")
	fs, err := toolkit.NewOsFS(jail, globVirtual)
	require.NoError(t, err)

	require.NoError(t, fs.WriteFile("a.txt", []byte("a"), 0o644))
	require.NoError(t, fs.WriteFile("b.txt", []byte("b"), 0o644))
	require.NoError(t, fs.WriteFile("c.md", []byte("c"), 0o644))

	relMatches, err := fs.Glob("*.txt")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"a.txt", "b.txt"}, relMatches)

	absMatches, err := fs.Glob(rootedPath("glob", "*.txt"))
	require.NoError(t, err)
	require.ElementsMatch(t, []string{rootedPath("glob", "a.txt"), rootedPath("glob", "b.txt")}, absMatches)
}

func TestOsFS_ResolvePath_FollowSymlinkEscape_Jailed(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	outside := t.TempDir()

	target := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(target, []byte("secret"), 0o644))

	linkPath := filepath.Join(jail, "out-link")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("skipping symlink test: symlink creation unavailable: %v", err)
	}

	fs, err := toolkit.NewOsFS(jail, rootedPath())
	require.NoError(t, err)

	_, err = fs.ResolvePath(rootedPath("out-link"), true)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
}
