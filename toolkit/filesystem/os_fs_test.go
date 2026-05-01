package filesystem_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jlrickert/cli-toolkit/toolkit/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOsFS_HostPath_InJailVirtualPath(t *testing.T) {
	t.Parallel()

	jailDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	fs, err := filesystem.NewOsFS(jailDir, "/")
	require.NoError(t, err)

	got, err := fs.HostPath("/home/alice/notes")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(jailDir, "home", "alice", "notes"), got)
}

func TestOsFS_HostPath_LexicalContract_ParentSymlinkNotChased(t *testing.T) {
	t.Parallel()

	// Plant a parent-traversal symlink: /jail/sneaky -> outside.
	// HostPath performs the same lexical IsInJail check as
	// resolveHostForCreate's parent-walk uses, but without the symlink
	// canonicalization of intermediate components — so the LEXICAL form
	// of an in-jail path stays in jail. This test confirms the lexical
	// jail-escape contract: paths that resolve outside the jail at the
	// LEXICAL layer return jail.ErrEscapeAttempt.
	//
	// Genuine parent-traversal-symlink defense (canonicalizing parents)
	// is a Phase 2 concern — Phase 1 HostPath only canonicalizes the
	// JAIL prefix to fix the macOS /var regression.
	jailDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	outsideDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	if err := os.Symlink(outsideDir, filepath.Join(jailDir, "sneaky")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	fs, err := filesystem.NewOsFS(jailDir, "/")
	require.NoError(t, err)

	// HostPath does NOT canonicalize the intermediate "sneaky" symlink
	// (Phase 1 contract), so the returned lexical host path stays under
	// the jail prefix even though OS-level traversal would escape. This
	// test asserts the documented Phase 1 behavior — Phase 2 will tighten
	// the contract.
	got, err := fs.HostPath("/sneaky/file.txt")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(jailDir, "sneaky", "file.txt"), got,
		"Phase 1 HostPath returns lexical jail-prefixed path; parent-symlink defense is Phase 2")
}

func TestOsFS_HostPath_NoJail(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	fs, err := filesystem.NewOsFS("", cwd)
	require.NoError(t, err)

	abs := filepath.Join(cwd, "some", "path")
	got, err := fs.HostPath(abs)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(abs), got)
}

func TestOsFS_HostPath_RelativeUsesWd(t *testing.T) {
	t.Parallel()

	jailDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	fs, err := filesystem.NewOsFS(jailDir, "/work")
	require.NoError(t, err)

	got, err := fs.HostPath("file.txt")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(jailDir, "work", "file.txt"), got)
}

// TestOsFS_HostPath_DarwinVarCanonicalization is the regression anchor
// for option 2b. On macOS, t.TempDir() lives under /var, which is itself
// a symlink to /private/var. Without canonicalizing the jail prefix in
// HostPath, downstream consumers that re-canonicalize would see the
// /private/var form and falsely flag the path as outside the jail.
func TestOsFS_HostPath_DarwinVarCanonicalization(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("regression test specific to macOS /var -> /private/var symlink")
	}
	t.Parallel()

	// Use the raw t.TempDir() (NOT EvalSymlinks'd) so the jail still
	// contains /var/...; HostPath itself must do the canonicalization.
	rawJail := t.TempDir()
	require.Contains(t, rawJail, "/var/", "expected macOS TempDir under /var")

	fs, err := filesystem.NewOsFS(rawJail, "/")
	require.NoError(t, err)

	got, err := fs.HostPath("/notes/keg")
	require.NoError(t, err)

	// The returned path must be under /private/var (the canonical form),
	// not /var (the symlink form), so that re-canonicalization by
	// callers stays inside the same jail prefix.
	canonicalJail, err := filepath.EvalSymlinks(rawJail)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(canonicalJail, "notes", "keg"), got)
	assert.Contains(t, got, "/private/var/",
		"expected canonical /private/var prefix, got %q", got)
}
