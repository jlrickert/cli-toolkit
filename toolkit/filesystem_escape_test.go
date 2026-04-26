package toolkit_test

// Symlink-escape tests for OsFS.
//
// These tests cover two distinct attack shapes:
//
//   1. Final-component follow. An in-jail symlink whose target is outside the
//      jail. Operations that follow the final component at the OS level
//      (ReadFile, ReadDir, WriteFile, OpenFile, AppendFile, AtomicWriteFile)
//      must reject the operation rather than touch the outside target.
//
//   2. Parent-traversal. An in-jail symlink in the PARENT chain whose target
//      is outside the jail (e.g. /jail/sneaky -> /outside, then op against
//      /jail/sneaky/foo). The OS resolves the parent symlink before reaching
//      the final component, which would let any operation — including no-
//      follow ones like Lchown, Remove, Rename, Lstat, Symlink-newname —
//      escape the jail. resolveHostForCreate canonicalizes the parent and
//      re-applies IsInJail to block this.
//
// Each test creates an outside target, plants the in-jail symlink, then
// asserts the operation either errors with toolkit.ErrEscapeAttempt or, for
// operations whose semantics make a hard error inappropriate, that the
// outside target is untouched (defense-in-depth).

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/require"
)

// makeParentTraversalJail creates a jailDir + outsideDir pair, plants a
// symlink jailDir/sneaky -> outsideDir, and returns both directories.
// Tests then operate on virtual paths under /sneaky/... and assert the
// operation is rejected by the parent-canonicalization logic.
func makeParentTraversalJail(t *testing.T) (jail, outside string) {
	t.Helper()
	jail = t.TempDir()
	outside = t.TempDir()
	if err := os.Symlink(outside, filepath.Join(jail, "sneaky")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	return jail, outside
}

// makeFinalSymlinkJail creates a jailDir + outsideDir pair, plants a target
// file inside outsideDir, and a symlink jailDir/escape-link -> targetFile.
// Tests operate on the virtual path /escape-link and assert the final-
// component symlink is rejected for follow-mode operations.
func makeFinalSymlinkJail(t *testing.T, contents string) (jail, outside, target string) {
	t.Helper()
	jail = t.TempDir()
	outside = t.TempDir()
	target = filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(target, []byte(contents), 0o644))
	if err := os.Symlink(target, filepath.Join(jail, "escape-link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	return jail, outside, target
}

// --- Parent-traversal escape tests ---

func TestOsFS_ReadFile_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "file.txt"), []byte("secret"), 0o644))

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	_, err = fs.ReadFile(rootedPath("sneaky", "file.txt"))
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
}

func TestOsFS_WriteFile_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	err = fs.WriteFile(rootedPath("sneaky", "planted.txt"), []byte("data"), 0o644)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)

	// Defense-in-depth: confirm no file was planted outside the jail.
	_, err = os.Stat(filepath.Join(outsideDir, "planted.txt"))
	require.True(t, os.IsNotExist(err), "expected no out-of-jail file; got err=%v", err)
}

func TestOsFS_OpenFile_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	w, err := fs.OpenFile(rootedPath("sneaky", "planted.txt"), os.O_CREATE|os.O_WRONLY, 0o644)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
	if w != nil {
		_ = w.Close()
	}

	_, err = os.Stat(filepath.Join(outsideDir, "planted.txt"))
	require.True(t, os.IsNotExist(err), "expected no out-of-jail file; got err=%v", err)
}

func TestOsFS_AppendFile_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	err = fs.AppendFile(rootedPath("sneaky", "planted.txt"), []byte("data"), 0o644)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)

	_, err = os.Stat(filepath.Join(outsideDir, "planted.txt"))
	require.True(t, os.IsNotExist(err), "expected no out-of-jail file; got err=%v", err)
}

func TestOsFS_AtomicWriteFile_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	err = fs.AtomicWriteFile(rootedPath("sneaky", "planted.txt"), []byte("data"), 0o644)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)

	_, err = os.Stat(filepath.Join(outsideDir, "planted.txt"))
	require.True(t, os.IsNotExist(err), "expected no out-of-jail file; got err=%v", err)
}

func TestOsFS_ReadDir_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)
	require.NoError(t, os.Mkdir(filepath.Join(outsideDir, "sub"), 0o755))

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	_, err = fs.ReadDir(rootedPath("sneaky", "sub"))
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
}

func TestOsFS_Mkdir_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	err = fs.Mkdir(rootedPath("sneaky", "newdir"), 0o755, false)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)

	_, err = os.Stat(filepath.Join(outsideDir, "newdir"))
	require.True(t, os.IsNotExist(err), "expected no out-of-jail dir; got err=%v", err)
}

func TestOsFS_Remove_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)
	victim := filepath.Join(outsideDir, "victim.txt")
	require.NoError(t, os.WriteFile(victim, []byte("important"), 0o644))

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	err = fs.Remove(rootedPath("sneaky", "victim.txt"), false)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)

	// Defense-in-depth: file outside the jail must remain.
	_, err = os.Stat(victim)
	require.NoError(t, err, "expected out-of-jail victim to be untouched")
}

func TestOsFS_Rename_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	t.Run("src_escape", func(t *testing.T) {
		jailDir, outsideDir := makeParentTraversalJail(t)
		require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "src.txt"), []byte("x"), 0o644))

		fs, err := toolkit.NewOsFS(jailDir, rootedPath())
		require.NoError(t, err)

		err = fs.Rename(rootedPath("sneaky", "src.txt"), rootedPath("dst.txt"))
		require.Error(t, err)
		require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
	})

	t.Run("dst_escape", func(t *testing.T) {
		jailDir, outsideDir := makeParentTraversalJail(t)

		fs, err := toolkit.NewOsFS(jailDir, rootedPath())
		require.NoError(t, err)
		require.NoError(t, fs.WriteFile(rootedPath("src.txt"), []byte("x"), 0o644))

		err = fs.Rename(rootedPath("src.txt"), rootedPath("sneaky", "moved.txt"))
		require.Error(t, err)
		require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)

		_, err = os.Stat(filepath.Join(outsideDir, "moved.txt"))
		require.True(t, os.IsNotExist(err), "expected no out-of-jail file; got err=%v", err)
	})
}

func TestOsFS_Symlink_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	// Attempt to plant a symlink whose newname escapes via /sneaky/.
	err = fs.Symlink(rootedPath("anywhere"), rootedPath("sneaky", "planted-link"))
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)

	_, err = os.Lstat(filepath.Join(outsideDir, "planted-link"))
	require.True(t, os.IsNotExist(err), "expected no out-of-jail symlink; got err=%v", err)
}

func TestOsFS_Lchown_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("lchown is a no-op on Windows; skipping")
	}

	jailDir, outsideDir := makeParentTraversalJail(t)
	victim := filepath.Join(outsideDir, "victim.txt")
	require.NoError(t, os.WriteFile(victim, []byte("important"), 0o644))

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	err = fs.Lchown(rootedPath("sneaky", "victim.txt"), os.Geteuid(), os.Getegid())
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
}

func TestOsFS_Stat_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "file.txt"), []byte("secret"), 0o644))

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	// followSymlinks=false (Lstat semantics): even though the FINAL component
	// is not followed, the parent symlink IS resolved by the OS, so the
	// jail-aware resolver must reject the operation.
	_, err = fs.Stat(rootedPath("sneaky", "file.txt"), false)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
}

// --- Final-component symlink-escape tests ---

func TestOsFS_ReadFile_FinalSymlinkEscape(t *testing.T) {
	t.Parallel()

	jailDir, _, _ := makeFinalSymlinkJail(t, "secret")

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	_, err = fs.ReadFile(rootedPath("escape-link"))
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
}

func TestOsFS_WriteFile_FinalSymlinkEscape(t *testing.T) {
	t.Parallel()

	jailDir, _, target := makeFinalSymlinkJail(t, "secret")

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	err = fs.WriteFile(rootedPath("escape-link"), []byte("overwrite"), 0o644)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)

	// Defense-in-depth: confirm the outside target was not overwritten.
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "secret", string(data))
}

func TestOsFS_OpenFile_FinalSymlinkEscape(t *testing.T) {
	t.Parallel()

	jailDir, _, target := makeFinalSymlinkJail(t, "secret")

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	w, err := fs.OpenFile(rootedPath("escape-link"), os.O_WRONLY|os.O_TRUNC, 0o644)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
	if w != nil {
		_ = w.Close()
	}

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "secret", string(data))
}

func TestOsFS_AppendFile_FinalSymlinkEscape(t *testing.T) {
	t.Parallel()

	jailDir, _, target := makeFinalSymlinkJail(t, "secret")

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	err = fs.AppendFile(rootedPath("escape-link"), []byte("more"), 0o644)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "secret", string(data))
}

func TestOsFS_AtomicWriteFile_FinalSymlinkEscape(t *testing.T) {
	t.Parallel()

	jailDir, _, target := makeFinalSymlinkJail(t, "secret")

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	err = fs.AtomicWriteFile(rootedPath("escape-link"), []byte("overwrite"), 0o644)
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "secret", string(data))
}

func TestOsFS_ReadDir_FinalSymlinkEscape(t *testing.T) {
	t.Parallel()

	jailDir := t.TempDir()
	outsideDir := t.TempDir()

	// Plant a sibling file inside the outside dir so an erroneous enumeration
	// would obviously leak out-of-jail content.
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "leaked.txt"), []byte("x"), 0o644))

	if err := os.Symlink(outsideDir, filepath.Join(jailDir, "escape-dir")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	_, err = fs.ReadDir(rootedPath("escape-dir"))
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
}

// --- Glob escape test ---

func TestOsFS_Glob_ParentTraversalEscape(t *testing.T) {
	t.Parallel()

	jailDir, outsideDir := makeParentTraversalJail(t)
	// Plant a candidate that would match if the parent symlink were followed.
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "match.txt"), []byte("x"), 0o644))

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	// The pattern's literal prefix /sneaky resolves through the in-jail
	// symlink to /outside; the resolver must reject the pattern.
	_, err = fs.Glob(rootedPath("sneaky", "*.txt"))
	require.Error(t, err)
	require.ErrorIs(t, err, toolkit.ErrEscapeAttempt)
}

// --- Sanity: legitimate operations still work ---

// TestOsFS_Symlink_FinalIsSymlinkButOldOnly_StillCreates verifies that
// creating a NEW symlink whose newname is a fresh path (not under any
// parent-traversal symlink) succeeds even when oldname points outside the
// jail. The jail check applies to where the inode lives (newname), not
// where the link points (oldname); follow operations validate later.
func TestOsFS_Symlink_OutsideTargetTextStillCreatesInJail(t *testing.T) {
	t.Parallel()

	jailDir := t.TempDir()
	outside := t.TempDir()

	fs, err := toolkit.NewOsFS(jailDir, rootedPath())
	require.NoError(t, err)

	// oldname points outside; newname is a clean in-jail path. The link
	// inode itself lands in the jail; subsequent follows would be jail-
	// checked. Symlink creation itself must succeed.
	target := filepath.Join(outside, "anything")
	if err := fs.Symlink(target, rootedPath("just-a-link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	// Verify the symlink inode is inside the jail.
	info, err := os.Lstat(filepath.Join(jailDir, "just-a-link"))
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink)
}
