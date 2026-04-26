package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jlrickert/cli-toolkit/toolkit/jail"
)

// OsFS is the canonical FileSystem implementation for host and jailed access.
//
// When jail is empty, paths resolve against the host filesystem. When jail is
// set, all virtual absolute paths are mapped under jail on the host.
type OsFS struct {
	mu sync.RWMutex

	jail string
	wd   string
}

// NewOsFS constructs an OsFS with optional jail and initial working directory.
//
// If wd is empty and jail is set, wd defaults to "/". If wd is empty and jail
// is not set, wd defaults to the process working directory.
func NewOsFS(jailPath, wd string) (*OsFS, error) {
	fs := &OsFS{}
	if err := fs.SetJail(jailPath); err != nil {
		return nil, err
	}

	initialWd := strings.TrimSpace(wd)
	if initialWd == "" {
		if strings.TrimSpace(fs.GetJail()) == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return nil, err
			}
			initialWd = cwd
		} else {
			initialWd = string(filepath.Separator)
		}
	}

	resolvedWd, err := fs.resolveVirtual(initialWd, false)
	if err != nil {
		return nil, err
	}
	fs.mu.Lock()
	fs.wd = resolvedWd
	fs.mu.Unlock()

	return fs, nil
}

func (fs *OsFS) ensureInitializedLocked() error {
	if strings.TrimSpace(fs.wd) != "" {
		return nil
	}
	if strings.TrimSpace(fs.jail) != "" {
		fs.wd = string(filepath.Separator)
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	fs.wd = filepath.Clean(cwd)
	return nil
}

func (fs *OsFS) GetJail() string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.jail
}

func (fs *OsFS) SetJail(jailPath string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if strings.TrimSpace(jailPath) == "" {
		fs.jail = ""
		return nil
	}
	fs.jail = filepath.Clean(jailPath)
	return nil
}

func (fs *OsFS) Getwd() (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if err := fs.ensureInitializedLocked(); err != nil {
		return "", err
	}
	return fs.wd, nil
}

func (fs *OsFS) Setwd(path string) error {
	resolved, err := fs.resolveVirtual(path, false)
	if err != nil {
		return err
	}

	host := fs.hostPath(resolved)
	info, err := os.Stat(host)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !info.IsDir() {
		return fmt.Errorf("setwd %q: not a directory", path)
	}

	fs.mu.Lock()
	fs.wd = resolved
	fs.mu.Unlock()
	return nil
}

func (fs *OsFS) ResolvePath(path string, followSymlinks bool) (string, error) {
	return fs.resolveVirtual(path, followSymlinks)
}

func (fs *OsFS) ReadFile(path string) ([]byte, error) {
	host, err := fs.resolveHost(path, false)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(host)
}

func (fs *OsFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	host, err := fs.resolveHost(path, false)
	if err != nil {
		return err
	}
	return os.WriteFile(host, data, perm)
}

func (fs *OsFS) Mkdir(path string, perm os.FileMode, all bool) error {
	host, err := fs.resolveHost(path, false)
	if err != nil {
		return err
	}
	if all {
		return os.MkdirAll(host, perm)
	}
	return os.Mkdir(host, perm)
}

func (fs *OsFS) Remove(path string, all bool) error {
	host, err := fs.resolveHost(path, false)
	if err != nil {
		return err
	}
	if all {
		return os.RemoveAll(host)
	}
	return os.Remove(host)
}

func (fs *OsFS) Rename(src, dst string) error {
	srcHost, err := fs.resolveHost(src, false)
	if err != nil {
		return err
	}
	dstHost, err := fs.resolveHost(dst, false)
	if err != nil {
		return err
	}
	return os.Rename(srcHost, dstHost)
}

func (fs *OsFS) Stat(path string, followSymlinks bool) (os.FileInfo, error) {
	// Pass followSymlinks through to resolveHost so the jail check runs
	// EvalSymlinks and re-checks IsInJail when the caller wants follow
	// semantics. Without this, an in-jail symlink to an outside target would
	// pass the lexical jail check and os.Stat would leak the target's
	// FileInfo. The followSymlinks=false path keeps lexical-only resolution
	// because os.Lstat does not follow symlinks at the OS level.
	host, err := fs.resolveHost(path, followSymlinks)
	if err != nil {
		return nil, err
	}
	if followSymlinks {
		return os.Stat(host)
	}
	return os.Lstat(host)
}

func (fs *OsFS) ReadDir(path string) ([]os.DirEntry, error) {
	host, err := fs.resolveHost(path, false)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(host)
}

func (fs *OsFS) Symlink(oldname, newname string) error {
	oldHost, err := fs.resolveHost(oldname, false)
	if err != nil {
		return err
	}
	newHost, err := fs.resolveHost(newname, false)
	if err != nil {
		return err
	}
	return os.Symlink(oldHost, newHost)
}

func (fs *OsFS) Glob(pattern string) ([]string, error) {
	wd, err := fs.Getwd()
	if err != nil {
		return nil, err
	}

	isRelative := !filepath.IsAbs(pattern)
	virtualPattern := pattern
	if isRelative {
		virtualPattern = filepath.Join(wd, pattern)
	}
	virtualPattern = filepath.Clean(virtualPattern)

	hostPattern := fs.hostPath(virtualPattern)
	matches, err := filepath.Glob(hostPattern)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(fs.GetJail()) == "" {
		return matches, nil
	}

	out := make([]string, 0, len(matches))
	jailPath := fs.GetJail()
	for _, m := range matches {
		if !jail.IsInJail(jailPath, m) {
			continue
		}
		virtual := jail.RemoveJailPrefix(jailPath, m)
		if isRelative {
			rel, err := filepath.Rel(wd, virtual)
			if err == nil {
				out = append(out, rel)
				continue
			}
		}
		out = append(out, virtual)
	}
	return out, nil
}

func (fs *OsFS) AppendFile(path string, data []byte, perm os.FileMode) error {
	host, err := fs.resolveHost(path, false)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(host, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (fs *OsFS) OpenFile(path string, flag int, perm os.FileMode) (io.WriteCloser, error) {
	host, err := fs.resolveHost(path, false)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(host, flag, perm)
}

func (fs *OsFS) Chmod(path string, mode os.FileMode) error {
	// Resolve with followSymlinks=true so resolveVirtual runs EvalSymlinks
	// and re-checks the resolved path against the jail. os.Chmod follows
	// symlinks at the OS level, so a lexical-only check would let a symlink
	// inside the jail mutate a target outside it.
	host, err := fs.resolveHost(path, true)
	if err != nil {
		return err
	}
	return os.Chmod(host, mode)
}

func (fs *OsFS) Chown(path string, uid, gid int) error {
	// See Chmod: os.Chown follows symlinks, so the jail check must be
	// symlink-aware to prevent escape via an in-jail link to an outside file.
	host, err := fs.resolveHost(path, true)
	if err != nil {
		return err
	}
	return os.Chown(host, uid, gid)
}

func (fs *OsFS) Lchown(path string, uid, gid int) error {
	// os.Lchown does NOT follow symlinks (POSIX), so a lexical jail check is
	// sufficient: the operation acts on the symlink inode itself, never its
	// target. Keeping followSymlinks=false here is intentional and required.
	host, err := fs.resolveHost(path, false)
	if err != nil {
		return err
	}
	return os.Lchown(host, uid, gid)
}

func (fs *OsFS) Chtimes(path string, atime, mtime time.Time) error {
	// See Chmod: os.Chtimes follows symlinks, so the jail check must be
	// symlink-aware to prevent escape via an in-jail link to an outside file.
	host, err := fs.resolveHost(path, true)
	if err != nil {
		return err
	}
	return os.Chtimes(host, atime, mtime)
}

func (fs *OsFS) AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	host, err := fs.resolveHost(path, false)
	if err != nil {
		return err
	}
	return atomicWriteFile(host, data, perm)
}

func (fs *OsFS) Rel(basePath, targetPath string) (string, error) {
	baseResolved, err := fs.resolveVirtual(basePath, false)
	if err != nil {
		return "", err
	}
	targetResolved, err := fs.resolveVirtual(targetPath, false)
	if err != nil {
		return "", err
	}
	return filepath.Rel(baseResolved, targetResolved)
}

func (fs *OsFS) resolveHost(path string, followSymlinks bool) (string, error) {
	resolved, err := fs.resolveVirtual(path, followSymlinks)
	if err != nil {
		return "", err
	}
	return fs.hostPath(resolved), nil
}

func (fs *OsFS) resolveVirtual(path string, followSymlinks bool) (string, error) {
	fs.mu.Lock()
	if err := fs.ensureInitializedLocked(); err != nil {
		fs.mu.Unlock()
		return "", err
	}
	wd := fs.wd
	jailPath := fs.jail
	fs.mu.Unlock()

	if wd == "" {
		wd = string(filepath.Separator)
	}

	if strings.TrimSpace(path) == "" || path == "." {
		path = wd
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(wd, path)
	}
	virtual := filepath.Clean(path)

	if jailPath == "" {
		if !followSymlinks {
			return virtual, nil
		}
		resolved, err := filepath.EvalSymlinks(virtual)
		if err != nil {
			return "", err
		}
		return filepath.Clean(resolved), nil
	}

	host := filepath.Clean(filepath.Join(jailPath, virtual))
	if !jail.IsInJail(jailPath, host) {
		return "", fmt.Errorf("resolve path outside jail %s: %w", host, jail.ErrEscapeAttempt)
	}

	if !followSymlinks {
		return virtual, nil
	}

	resolvedHost, err := filepath.EvalSymlinks(host)
	if err != nil {
		return "", err
	}

	// Canonicalize the jail prefix so the IsInJail comparison is meaningful
	// after EvalSymlinks. On systems where the jail's parent contains
	// symlinks (e.g. macOS where /var -> /private/var), the resolved host
	// path is in canonical form while the stored jailPath is not, and a
	// raw prefix comparison would falsely flag legitimate paths as escapes.
	// Fall back to the raw jailPath if EvalSymlinks fails (jail may not yet
	// exist at construction time).
	canonicalJail := jailPath
	if evaledJail, evalErr := filepath.EvalSymlinks(jailPath); evalErr == nil {
		canonicalJail = evaledJail
	}

	if !jail.IsInJail(canonicalJail, resolvedHost) {
		return "", fmt.Errorf("resolve path outside jail %s: %w", resolvedHost, jail.ErrEscapeAttempt)
	}
	return filepath.Clean(jail.RemoveJailPrefix(canonicalJail, resolvedHost)), nil
}

func (fs *OsFS) hostPath(path string) string {
	jailPath := fs.GetJail()
	if jailPath == "" {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(jailPath, path))
}

var _ FileSystem = (*OsFS)(nil)
