package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jlrickert/cli-toolkit/toolkit/jail"
)

// FileSystem defines the contract for filesystem operations operating on
// resolved/host filesystem paths.
//
// Path and pattern inputs may be absolute or relative. Relative inputs are
// resolved against the implementation's current working directory as reported
// by Getwd and changed by Setwd.
type FileSystem interface {
	jail.Jailed

	// ReadFile reads the contents of the file at path.
	// Relative paths are resolved from the current working directory.
	ReadFile(path string) ([]byte, error)
	// WriteFile writes data to path with the provided permissions.
	// Relative paths are resolved from the current working directory.
	WriteFile(path string, data []byte, perm os.FileMode) error
	// Mkdir creates a directory at path, using MkdirAll when all is true.
	// Relative paths are resolved from the current working directory.
	Mkdir(path string, perm os.FileMode, all bool) error
	// Remove deletes path, using recursive removal when all is true.
	// Relative paths are resolved from the current working directory.
	Remove(path string, all bool) error
	// Rename moves or renames src to dst.
	// Relative src and dst paths are resolved from the current working directory.
	Rename(src, dst string) error
	// Stat returns file metadata for path, following symlinks when requested.
	// Relative paths are resolved from the current working directory.
	Stat(path string, followSymlinks bool) (os.FileInfo, error)
	// ReadDir reads and returns directory entries for path.
	// Relative paths are resolved from the current working directory.
	ReadDir(path string) ([]os.DirEntry, error)
	// Symlink creates newname as a symbolic link to oldname.
	// Relative oldname and newname paths are resolved from the current working directory.
	Symlink(oldname, newname string) error
	// Glob returns paths matching the provided pattern.
	// Relative patterns are evaluated from the current working directory.
	Glob(pattern string) ([]string, error)
	// AppendFile appends data to path, creating the file if it does not exist.
	// Relative paths are resolved from the current working directory.
	AppendFile(path string, data []byte, perm os.FileMode) error
	// OpenFile opens the file at path with the given flags and permissions,
	// returning an io.WriteCloser. Callers must close the returned writer.
	// Relative paths are resolved from the current working directory.
	OpenFile(path string, flag int, perm os.FileMode) (io.WriteCloser, error)
	// Chmod changes the mode of the file at path to mode.
	// Relative paths are resolved from the current working directory.
	// On Windows only the read-only bit is honored, matching the
	// behavior of WriteFile's perm argument.
	Chmod(path string, mode os.FileMode) error
	// Chown changes the numeric uid and gid of the file at path.
	// Relative paths are resolved from the current working directory.
	// On Windows this is a no-op.
	Chown(path string, uid, gid int) error
	// Lchown is like Chown but does not follow symlinks.
	// Relative paths are resolved from the current working directory.
	// On Windows this is a no-op.
	Lchown(path string, uid, gid int) error
	// Chtimes changes the access and modification times of the file at path.
	// Relative paths are resolved from the current working directory.
	// On Windows the resolution of access time is filesystem-dependent
	// (e.g. FAT32 truncates to 2-second granularity).
	Chtimes(path string, atime, mtime time.Time) error
	// AtomicWriteFile writes data to path atomically with the provided permissions.
	// Relative paths are resolved from the current working directory.
	AtomicWriteFile(path string, data []byte, perm os.FileMode) error
	// Rel returns a relative path from basePath to targetPath.
	// Relative paths are resolved from the current working directory.
	Rel(basePath, targetPath string) (string, error)
	// Getwd returns the current working directory used for relative path resolution.
	Getwd() (string, error)
	// Setwd sets the current working directory to path.
	// Relative paths are resolved from the current working directory.
	Setwd(path string) error
	// ResolvePath resolves path to an absolute normalized path, optionally following symlinks.
	// Relative paths are resolved from the current working directory.
	ResolvePath(path string, followSymlinks bool) (string, error)
	// HostPath translates a virtual (jail-relative) path into the
	// equivalent absolute host filesystem path that an external program
	// (subprocess, editor, etc.) needs to operate on.
	//
	// Implementations canonicalize the jail prefix via filepath.EvalSymlinks
	// before joining so platform-level symlinks (e.g. macOS /var ->
	// /private/var) do not produce phantom escapes when callers later
	// re-canonicalize the returned path. Implementations do NOT
	// canonicalize intermediate symlinks in the virtual path; callers
	// that need parent-traversal-symlink defense should resolve the
	// path through ResolvePath(_, true) first.
	//
	// HostPath is the public surface of the in-tree host-translation
	// helper used by the FileSystem implementation itself. It is
	// intended for callers that need to hand a host path to code outside
	// the FileSystem abstraction (e.g. exec.Command).
	//
	// The input may be absolute (treated as virtual when a jail is
	// configured) or relative (resolved against the current working
	// directory). Returns [jail.ErrEscapeAttempt] when the lexical jail
	// check rejects the resulting path. When no jail is configured, the
	// cleaned absolute host path is returned unchanged.
	HostPath(virtual string) (string, error)
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("atomic write: mkdirall %q: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("atomic write: create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("atomic write: write temp file %q: %w", tmpName, err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("atomic write: close temp file %q: %w", tmpName, err)
	}

	if err := os.Chmod(tmpName, perm); err != nil {
		// Not fatal: continue anyway.
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomic write: rename %q -> %q: %w", tmpName, path, err)
	}

	return nil
}
