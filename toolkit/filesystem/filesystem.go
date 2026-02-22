package filesystem

import (
	"fmt"
	"os"
	"path/filepath"

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
