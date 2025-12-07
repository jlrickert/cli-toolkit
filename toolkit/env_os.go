package toolkit

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// OsEnv is an Env implementation that delegates to the real process
// environment and filesystem. Use this in production code where access to the
// actual OS environment is required.
type OsEnv struct{}

func (o *OsEnv) Name() string {
	return "os"
}

// GetHome returns the home directory reported by the OS. It delegates to
// os.UserHomeDir.
func (o *OsEnv) GetHome() (string, error) {
	return os.UserHomeDir()
}

// SetHome sets environment values that represent the user's home directory.
//
// On Windows it also sets USERPROFILE to satisfy common callers.
func (o *OsEnv) SetHome(home string) error {
	if runtime.GOOS == "windows" {
		if err := os.Setenv("USERPROFILE", home); err != nil {
			return err
		}
	}
	return os.Setenv("HOME", home)
}

// GetUser returns the current OS user username. If the Username field is
// empty it falls back to the user's Name field.
func (o *OsEnv) GetUser() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	// Username is typically what callers expect; fall back to Name if empty.
	if u.Username != "" {
		return u.Username, nil
	}
	return u.Name, nil
}

// SetUser sets environment values that represent the current user.
//
// On Windows it also sets USERNAME in addition to USER.
func (o *OsEnv) SetUser(username string) error {
	if runtime.GOOS == "windows" {
		if err := os.Setenv("USERNAME", username); err != nil {
			return err
		}
	}
	return os.Setenv("USER", username)
}

// Get returns the environment variable for key.
func (o *OsEnv) Get(key string) string {
	return os.Getenv(key)
}

// Set sets the OS environment variable key to value.
func (o *OsEnv) Set(key string, value string) error {
	return os.Setenv(key, value)
}

// Environ returns a copy of the process environment in "key=value" form.
func (o *OsEnv) Environ() []string {
	return os.Environ()
}

// Has reports whether the given environment key is present.
func (o *OsEnv) Has(key string) bool {
	_, ok := os.LookupEnv(key)
	return ok
}

// Unset removes the OS environment variable.
func (o *OsEnv) Unset(key string) {
	_ = os.Unsetenv(key)
}

// GetTempDir returns the OS temporary directory.
func (o *OsEnv) GetTempDir() string {
	return os.TempDir()
}

// Getwd returns the current process working directory.
func (o *OsEnv) Getwd() (string, error) {
	return os.Getwd()
}

// Setwd attempts to change the process working directory to dir.
//
// It also attempts to update PWD. Chdir errors are intentionally ignored to
// avoid surprising callers.
func (o *OsEnv) Setwd(dir string) {
	p, _ := filepath.Abs(dir)
	// _ = os.Setenv("PWD", p)
	_ = os.Chdir(p)
}

func (o *OsEnv) ExpandPath(p string) string {
	if p == "" {
		return p
	}
	if p[0] != '~' {
		return p
	}

	// Only expand the simple leading tilde forms: "~" or "~/" or "~\"
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, _ := o.GetHome()
		if p == "~" {
			return filepath.Clean(home)
		}
		// Trim the "~/" or "~\" prefix and join with home to produce a
		// well formed path.
		rest := p[2:]
		return filepath.Join(home, rest)
	}

	// More complex cases like "~username/..." are not supported and are
	// returned unchanged.
	return p
}

// ReadFile reads the named file from the real filesystem.
func (o *OsEnv) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// WriteFile writes data to a file on the real filesystem with the given
// permissions.
func (o *OsEnv) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// Remove removes the named file or directory. If all is true all items in the
// path are removed.
func (o *OsEnv) Remove(path string, all bool) error {
	if all {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// Rename renames (moves) a file or directory.
func (o *OsEnv) Rename(src, dst string) error {
	return os.Rename(src, dst)
}

// Mkdir creates a directory. If all is true MkdirAll is used.
func (o *OsEnv) Mkdir(path string, perm os.FileMode, all bool) error {
	if all {
		return os.MkdirAll(path, perm)
	}
	return os.Mkdir(path, perm)
}

func (o *OsEnv) ReadDir(rel string) ([]os.DirEntry, error) {
	return os.ReadDir(rel)
}

// ResolvePath implements FileSystem.
func (o *OsEnv) ResolvePath(rel string, follow bool) (string, error) {
	p := o.ExpandPath(rel)
	if filepath.IsAbs(p) {
		if follow {
			return filepath.EvalSymlinks(p)
		}
		return p, nil
	}

	cwd, err := o.Getwd()
	if err != nil {
		return cwd, err
	}
	abs := filepath.Join(cwd, p)
	if follow {
		return filepath.EvalSymlinks(abs)
	}
	return abs, nil
}

func (o *OsEnv) Stat(name string, follow bool) (os.FileInfo, error) {
	path, err := o.ResolvePath(name, follow)
	if err != nil {
		return nil, err
	}
	return os.Stat(path)
}

func (o *OsEnv) Symlink(oldname string, newname string) error {
	oldPath := o.ExpandPath(oldname)
	newPath := o.ExpandPath(newname)
	return os.Symlink(oldPath, newPath)
}

func (o *OsEnv) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

func (o *OsEnv) AtomicWriteFile(rel string, data []byte, perm os.FileMode) error {
	path := o.ExpandPath(rel)

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
		// Not fatal: continue anyway
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomic write: rename %q -> %q: %w", tmpName, path, err)
	}

	return nil
}

// Ensure implementations satisfy the interfaces.
var _ Env = (*OsEnv)(nil)
var _ FileSystem = (*OsEnv)(nil)
