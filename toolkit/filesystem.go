package toolkit

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileSystem defines the contract for filesystem operations operating on
// resolved/host filesystem paths.
type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	Mkdir(path string, perm os.FileMode, all bool) error
	Remove(path string, all bool) error
	Rename(src, dst string) error
	Stat(path string, followSymlinks bool) (os.FileInfo, error)
	ReadDir(path string) ([]os.DirEntry, error)
	Symlink(oldname, newname string) error
	Glob(pattern string) ([]string, error)
	AtomicWriteFile(path string, data []byte, perm os.FileMode) error
}

// OsFS is a FileSystem implementation backed by the host OS filesystem.
type OsFS struct{}

func (o *OsFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (o *OsFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (o *OsFS) Mkdir(path string, perm os.FileMode, all bool) error {
	if all {
		return os.MkdirAll(path, perm)
	}
	return os.Mkdir(path, perm)
}

func (o *OsFS) Remove(path string, all bool) error {
	if all {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

func (o *OsFS) Rename(src, dst string) error {
	return os.Rename(src, dst)
}

func (o *OsFS) Stat(path string, followSymlinks bool) (os.FileInfo, error) {
	if followSymlinks {
		return os.Stat(path)
	}
	return os.Lstat(path)
}

func (o *OsFS) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func (o *OsFS) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (o *OsFS) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

func (o *OsFS) AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("atomic write: mkdirall %q: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp("", ".tmp-"+filepath.Base(path)+".*")
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

var _ FileSystem = (*OsFS)(nil)
