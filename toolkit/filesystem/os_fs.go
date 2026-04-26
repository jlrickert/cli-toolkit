package filesystem

import (
	"errors"
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
	// ReadFile must reject final-component symlinks pointing outside the jail
	// (os.ReadFile follows symlinks at the OS level), so we use
	// followSymlinks=true. resolveHost(path, true) runs EvalSymlinks on the
	// full path and re-checks IsInJail on the canonicalized result.
	host, err := fs.resolveHost(path, true)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(host)
}

func (fs *OsFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	// os.WriteFile follows an existing final-component symlink, so we need
	// resolveHostForOpen rather than plain resolveHostForCreate: it adds the
	// final-symlink check on top of parent canonicalization and rejects
	// /jail/sneaky -> /outside/secret before any data is written.
	host, err := fs.resolveHostForOpen(path)
	if err != nil {
		return err
	}
	return os.WriteFile(host, data, perm)
}

func (fs *OsFS) Mkdir(path string, perm os.FileMode, all bool) error {
	// Mkdir creates the final component; use the parent-canonicalizing
	// resolver to block parent-traversal escapes through symlinked dirs.
	host, err := fs.resolveHostForCreate(path)
	if err != nil {
		return err
	}
	if all {
		return os.MkdirAll(host, perm)
	}
	return os.Mkdir(host, perm)
}

func (fs *OsFS) Remove(path string, all bool) error {
	// Remove operates on the final component without following it (POSIX
	// unlink/rmdir do not follow), but parent-component symlinks would let an
	// attacker delete files outside the jail. resolveHostForCreate
	// canonicalizes the parent so /jail/sneaky -> /outside, then
	// Remove(/jail/sneaky/foo) is rejected.
	host, err := fs.resolveHostForCreate(path)
	if err != nil {
		return err
	}
	if all {
		return os.RemoveAll(host)
	}
	return os.Remove(host)
}

func (fs *OsFS) Rename(src, dst string) error {
	// Both endpoints must canonicalize parents to block parent-traversal
	// escapes. The final components are not followed by os.Rename.
	srcHost, err := fs.resolveHostForCreate(src)
	if err != nil {
		return err
	}
	dstHost, err := fs.resolveHostForCreate(dst)
	if err != nil {
		return err
	}
	return os.Rename(srcHost, dstHost)
}

func (fs *OsFS) Stat(path string, followSymlinks bool) (os.FileInfo, error) {
	// followSymlinks=true: resolveHost runs EvalSymlinks on the full path so
	// an in-jail symlink to an outside target is rejected (otherwise os.Stat
	// would leak the target's FileInfo).
	//
	// followSymlinks=false: os.Lstat does not follow the FINAL component, but
	// parent-traversal symlinks would still let the OS resolve through them.
	// resolveHostForCreate canonicalizes the parent and re-checks IsInJail,
	// blocking the parent-traversal escape while leaving the final component
	// alone for Lstat to inspect as-is.
	if followSymlinks {
		host, err := fs.resolveHost(path, true)
		if err != nil {
			return nil, err
		}
		return os.Stat(host)
	}
	host, err := fs.resolveHostForCreate(path)
	if err != nil {
		return nil, err
	}
	return os.Lstat(host)
}

func (fs *OsFS) ReadDir(path string) ([]os.DirEntry, error) {
	// os.ReadDir follows symlinks (it stats the target to enumerate dir
	// entries). Use followSymlinks=true so a final-component symlink to an
	// outside dir is rejected, not silently enumerated.
	host, err := fs.resolveHost(path, true)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(host)
}

func (fs *OsFS) Symlink(oldname, newname string) error {
	// oldname is the symlink's target text. Resolve it lexically (no parent
	// canonicalization, no EvalSymlinks) so the resulting symlink works
	// inside the jail: a virtual target like "/home/x/file" must be stored
	// as the host path "<jail>/home/x/file" or later follow operations would
	// look outside the jail. resolveHost(_, false) does exactly this lexical
	// jail-to-host translation. The target need not exist at link creation.
	//
	// newname is where the symlink INODE is created. It must use
	// resolveHostForCreate so a parent-traversal escape — e.g.
	// /jail/sneaky -> /outside, then Symlink(target, /jail/sneaky/foo) —
	// is rejected before os.Symlink plants the link in the wrong place.
	oldHost, err := fs.resolveHost(oldname, false)
	if err != nil {
		return err
	}
	newHost, err := fs.resolveHostForCreate(newname)
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

	// Block parent-traversal escapes in the pattern's literal prefix. Split
	// the pattern at the first glob meta-character; the segment before it is
	// the literal directory prefix that filepath.Glob will traverse. We
	// canonicalize the WHOLE prefix (resolveHost with followSymlinks=true)
	// because Glob's semantics imply the prefix must already exist as a
	// directory; if the prefix itself is a symlink to outside, every match
	// would be out-of-jail and the per-match filter below would silently
	// drop everything. Erroring early gives a clearer signal.
	if jailPath := fs.GetJail(); strings.TrimSpace(jailPath) != "" {
		literalPrefix := globLiteralPrefix(virtualPattern)
		if literalPrefix != "" && literalPrefix != string(filepath.Separator) {
			if _, perr := fs.resolveHost(literalPrefix, true); perr != nil {
				// Only short-circuit on jail-escape errors; other errors
				// (e.g. the prefix legitimately not existing yet) should let
				// filepath.Glob proceed and return its empty result.
				if errors.Is(perr, jail.ErrEscapeAttempt) {
					return nil, perr
				}
			}
		}
	}

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
	// O_APPEND|O_CREATE|O_WRONLY follows an existing final-component symlink,
	// so use resolveHostForOpen to block both parent-traversal AND final-
	// component symlink escapes.
	host, err := fs.resolveHostForOpen(path)
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
	// os.OpenFile follows an existing final-component symlink unless the
	// caller passes O_NOFOLLOW. We treat all callers as if they may follow,
	// so resolveHostForOpen runs the parent canonicalization plus the final-
	// symlink check, blocking both escape shapes.
	host, err := fs.resolveHostForOpen(path)
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
	// os.Lchown does NOT follow symlinks on the FINAL component (POSIX), so
	// the operation acts on the symlink inode itself rather than its target.
	// However, parent-component symlinks ARE resolved by the OS, so we must
	// canonicalize the parent through resolveHostForCreate to block escapes
	// of the form /jail/sneaky -> /outside, then Lchown(/jail/sneaky/foo).
	host, err := fs.resolveHostForCreate(path)
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
	// AtomicWriteFile writes to a temp file in the same directory then
	// renames into place. The rename overwrites any existing final-component
	// symlink (replacing it with a regular file) but the temp file creation
	// happens in the parent dir, which must be canonicalized to defeat
	// parent-traversal escapes. Use resolveHostForOpen so the final-symlink
	// check rejects /jail/sneaky -> /outside before any temp file is even
	// created in the wrong place.
	host, err := fs.resolveHostForOpen(path)
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

// resolveHostForCreate returns a host path safe for operations whose final
// component (a) may not exist yet (create-mode) or (b) must NOT be followed
// at the OS level. It canonicalizes the parent directory via EvalSymlinks
// and re-checks IsInJail on the canonicalized parent, then re-attaches the
// original base name. This blocks the parent-traversal escape shape:
//
//	/jail/sneaky -> /outside  (symlink in jail)
//	op(/jail/sneaky/foo)      (OS resolves the parent symlink before reaching foo)
//
// For methods that DO follow the final symlink at the OS level AND require
// the final to exist (e.g. Chmod, ReadFile, ReadDir, Stat-with-follow), use
// resolveHost(path, true) instead. For open-mode methods that may create but
// also follow an existing final symlink (WriteFile, OpenFile, AppendFile,
// AtomicWriteFile), use resolveHostForOpen, which adds a final-symlink check
// on top of resolveHostForCreate.
//
// When the parent does not yet exist, the helper walks up the chain to find
// the longest existing ancestor, EvalSymlinks that, and re-attaches the
// missing intermediate segments lexically. This covers MkdirAll-style cases
// where several intermediate directories are about to be created.
func (fs *OsFS) resolveHostForCreate(path string) (string, error) {
	// Resolve virtually first to get jail-relative form and run the lexical
	// IsInJail check against the unresolved path. resolveVirtual with
	// followSymlinks=false never errors on missing components.
	resolved, err := fs.resolveVirtual(path, false)
	if err != nil {
		return "", err
	}
	host := fs.hostPath(resolved)

	jailPath := fs.GetJail()
	if strings.TrimSpace(jailPath) == "" {
		// No jail: nothing to enforce. Return the lexical host path.
		return host, nil
	}

	// Canonicalize the jail prefix once so the post-canonicalization
	// IsInJail check is meaningful even when the jail's parent contains
	// symlinks (e.g. macOS /var -> /private/var). Fall back to raw jail if
	// EvalSymlinks fails (jail may not exist yet).
	canonicalJail := jailPath
	if evaledJail, evalErr := filepath.EvalSymlinks(jailPath); evalErr == nil {
		canonicalJail = evaledJail
	}

	// Special case: host == jail root. Nothing to canonicalize; return as-is.
	if filepath.Clean(host) == filepath.Clean(jailPath) {
		return host, nil
	}

	parent := filepath.Dir(host)
	base := filepath.Base(host)

	// Walk up from parent to find the longest existing ancestor. We need an
	// existing path to EvalSymlinks; we will reattach the missing tail
	// segments lexically. This handles MkdirAll where several parents are
	// about to be created at once.
	missing := []string{base}
	cur := parent
	for {
		if _, err := os.Lstat(cur); err == nil {
			break
		} else if !os.IsNotExist(err) {
			// Permission denied or other I/O error: do not silently treat as
			// missing; bubble up so the caller sees the real failure.
			return "", err
		}
		// cur does not exist; record its base and walk up.
		next := filepath.Dir(cur)
		if next == cur {
			// Reached filesystem root without finding an existing ancestor.
			// This should not happen since the host root always exists, but
			// guard against an infinite loop.
			break
		}
		missing = append([]string{filepath.Base(cur)}, missing...)
		cur = next
	}

	// EvalSymlinks the existing ancestor. If it fails, fall back to the
	// lexical ancestor; the IsInJail check below will still run.
	canonicalAncestor := cur
	if evaled, evalErr := filepath.EvalSymlinks(cur); evalErr == nil {
		canonicalAncestor = evaled
	}

	if !jail.IsInJail(canonicalJail, canonicalAncestor) {
		return "", fmt.Errorf("resolve path outside jail %s: %w", canonicalAncestor, jail.ErrEscapeAttempt)
	}

	// Reattach missing segments lexically. Since the canonicalized ancestor
	// is in jail and the segments are simple base names (no separators), the
	// reassembled path stays in jail.
	out := canonicalAncestor
	for _, seg := range missing {
		out = filepath.Join(out, seg)
	}
	return filepath.Clean(out), nil
}

// resolveHostForOpen returns a host path safe for operations that may create
// the final component but ALSO follow an existing final-component symlink
// at the OS level (WriteFile, OpenFile, AppendFile, AtomicWriteFile).
//
// It first runs resolveHostForCreate to canonicalize the parent, then, if
// the final component exists and is a symlink, runs EvalSymlinks on the
// full path and re-checks IsInJail on the canonicalized result. This blocks
// the final-component-symlink escape:
//
//	/jail/sneaky -> /outside/secret  (symlink in jail)
//	WriteFile(/jail/sneaky, data)    (os.WriteFile follows -> writes /outside/secret)
//
// Non-existent finals and non-symlink finals are returned with parent
// canonicalization only.
func (fs *OsFS) resolveHostForOpen(path string) (string, error) {
	host, err := fs.resolveHostForCreate(path)
	if err != nil {
		return "", err
	}

	jailPath := fs.GetJail()
	if strings.TrimSpace(jailPath) == "" {
		return host, nil
	}

	info, statErr := os.Lstat(host)
	if statErr != nil {
		// Final does not exist (or stat failed): create-mode is fine, parent
		// canonicalization already enforced the jail. Return as-is.
		return host, nil
	}
	if info.Mode()&os.ModeSymlink == 0 {
		// Regular file or dir: no final-component follow happens at the OS
		// level beyond the parent (which is already canonical). Safe.
		return host, nil
	}

	// Final is a symlink and the operation will follow it. EvalSymlinks the
	// full host path and re-check IsInJail.
	canonicalJail := jailPath
	if evaledJail, evalErr := filepath.EvalSymlinks(jailPath); evalErr == nil {
		canonicalJail = evaledJail
	}

	resolvedHost, err := filepath.EvalSymlinks(host)
	if err != nil {
		return "", err
	}
	if !jail.IsInJail(canonicalJail, resolvedHost) {
		return "", fmt.Errorf("resolve path outside jail %s: %w", resolvedHost, jail.ErrEscapeAttempt)
	}
	return resolvedHost, nil
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

// globLiteralPrefix returns the longest leading path segment of pattern that
// contains no glob meta-characters (*, ?, [, \). This is the directory
// prefix filepath.Glob will traverse before matching. Returns "" if the very
// first segment contains a meta-character.
func globLiteralPrefix(pattern string) string {
	// Walk the pattern and stop at the first meta-character; trim back to the
	// last separator to get a clean directory prefix. If no meta-character
	// is found, the whole pattern is literal — return its parent (the dir
	// portion) so callers consistently get a directory path.
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '?', '[', '\\':
			// Trim back to the last separator before i.
			j := strings.LastIndex(pattern[:i], string(filepath.Separator))
			if j < 0 {
				return ""
			}
			return pattern[:j]
		}
	}
	return filepath.Dir(pattern)
}

func (fs *OsFS) hostPath(path string) string {
	jailPath := fs.GetJail()
	if jailPath == "" {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(jailPath, path))
}

var _ FileSystem = (*OsFS)(nil)
