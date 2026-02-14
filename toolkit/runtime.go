package toolkit

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jlrickert/cli-toolkit/clock"
	"github.com/jlrickert/cli-toolkit/mylog"
)

// Runtime is the explicit dependency container for commands and helpers.
//
// Context values are not used for mutable runtime dependencies; callers pass a
// Runtime directly.
type Runtime struct {
	Env    Env
	FS     FileSystem
	Clock  clock.Clock
	Logger *slog.Logger
	Stream *Stream
	Hasher Hasher

	// Jail is an optional host path used to confine filesystem operations.
	Jail string
}

// RuntimeOption mutates Runtime construction.
type RuntimeOption func(*Runtime) error

// NewRuntime constructs a Runtime with defaults and applies options.
func NewRuntime(opts ...RuntimeOption) (*Runtime, error) {
	rt := &Runtime{
		Env:    &OsEnv{},
		FS:     &OsFS{},
		Clock:  &clock.OsClock{},
		Logger: mylog.NewDiscardLogger(),
		Stream: DefaultStream(),
		Hasher: DefaultHasher,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(rt); err != nil {
			return nil, err
		}
	}

	if err := rt.Validate(); err != nil {
		return nil, err
	}

	return rt, nil
}

func WithRuntimeEnv(env Env) RuntimeOption {
	return func(rt *Runtime) error {
		if env == nil {
			return fmt.Errorf("runtime env cannot be nil")
		}
		rt.Env = env
		return nil
	}
}

func WithRuntimeFileSystem(fs FileSystem) RuntimeOption {
	return func(rt *Runtime) error {
		if fs == nil {
			return fmt.Errorf("runtime filesystem cannot be nil")
		}
		rt.FS = fs
		return nil
	}
}

func WithRuntimeClock(c clock.Clock) RuntimeOption {
	return func(rt *Runtime) error {
		if c == nil {
			return fmt.Errorf("runtime clock cannot be nil")
		}
		rt.Clock = c
		return nil
	}
}

func WithRuntimeLogger(lg *slog.Logger) RuntimeOption {
	return func(rt *Runtime) error {
		if lg == nil {
			return fmt.Errorf("runtime logger cannot be nil")
		}
		rt.Logger = lg
		return nil
	}
}

func WithRuntimeStream(s *Stream) RuntimeOption {
	return func(rt *Runtime) error {
		if s == nil {
			return fmt.Errorf("runtime stream cannot be nil")
		}
		rt.Stream = s
		return nil
	}
}

func WithRuntimeHasher(h Hasher) RuntimeOption {
	return func(rt *Runtime) error {
		if h == nil {
			return fmt.Errorf("runtime hasher cannot be nil")
		}
		rt.Hasher = h
		return nil
	}
}

func WithRuntimeJail(jail string) RuntimeOption {
	return func(rt *Runtime) error {
		rt.Jail = filepath.Clean(jail)
		return nil
	}
}

// Validate ensures required runtime dependencies are present.
func (rt *Runtime) Validate() error {
	if rt == nil {
		return fmt.Errorf("runtime is nil")
	}
	if rt.Env == nil {
		return fmt.Errorf("runtime env is nil")
	}
	if rt.FS == nil {
		return fmt.Errorf("runtime filesystem is nil")
	}
	if rt.Clock == nil {
		return fmt.Errorf("runtime clock is nil")
	}
	if rt.Logger == nil {
		return fmt.Errorf("runtime logger is nil")
	}
	if rt.Stream == nil {
		return fmt.Errorf("runtime stream is nil")
	}
	if rt.Hasher == nil {
		return fmt.Errorf("runtime hasher is nil")
	}
	return nil
}

// Clone returns a shallow clone of the runtime and a deep clone of Env/Stream
// when supported.
func (rt *Runtime) Clone() *Runtime {
	if rt == nil {
		return nil
	}

	clone := *rt

	if rt.Env != nil {
		if cloner, ok := rt.Env.(EnvCloner); ok {
			clone.Env = cloner.CloneEnv()
		}
	}

	if rt.Stream != nil {
		streamCopy := *rt.Stream
		clone.Stream = &streamCopy
	}

	return &clone
}

func (rt *Runtime) hostPath(path string) string {
	if rt.Jail == "" {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(rt.Jail, path))
}

// AbsPath returns a cleaned absolute runtime path.
//
// The returned path is relative to the runtime jail root when a jail is set
// (e.g. "/home/testuser/file").
func (rt *Runtime) AbsPath(rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", nil
	}
	if err := rt.Validate(); err != nil {
		return "", err
	}

	p, err := ExpandPath(rt.Env, rel)
	if err != nil {
		return "", err
	}

	if !filepath.IsAbs(p) {
		cwd, err := rt.Env.Getwd()
		if err != nil {
			return "", err
		}
		p = filepath.Join(cwd, p)
	}

	p = filepath.Clean(p)

	if rt.Jail == "" {
		return p, nil
	}

	host := rt.hostPath(p)
	if !IsInJail(rt.Jail, host) {
		return "", fmt.Errorf("AbsPath outside of jail %s: %w", host, ErrEscapeAttempt)
	}
	return RemoveJailPrefix(rt.Jail, host), nil
}

// ResolvePath resolves rel to an absolute runtime path and optionally follows
// symlinks.
func (rt *Runtime) ResolvePath(rel string, follow bool) (string, error) {
	if strings.TrimSpace(rel) == "" || rel == "." {
		if err := rt.Validate(); err != nil {
			return "", err
		}
		wd, err := rt.Env.Getwd()
		if err != nil {
			return "", err
		}
		if rt.Jail != "" {
			host := rt.hostPath(wd)
			if !IsInJail(rt.Jail, host) {
				return "", fmt.Errorf("ResolvePath outside of jail %s: %w", host, ErrEscapeAttempt)
			}
			if follow {
				resolved, err := filepath.EvalSymlinks(host)
				if err != nil {
					return "", err
				}
				if !IsInJail(rt.Jail, resolved) {
					return "", fmt.Errorf("ResolvePath outside of jail %s: %w", resolved, ErrEscapeAttempt)
				}
				return RemoveJailPrefix(rt.Jail, resolved), nil
			}
			return RemoveJailPrefix(rt.Jail, host), nil
		}
		return filepath.Clean(wd), nil
	}

	path, err := rt.AbsPath(rel)
	if err != nil {
		return "", err
	}
	if !follow {
		return filepath.Clean(path), nil
	}

	host := rt.hostPath(path)
	resolved, err := filepath.EvalSymlinks(host)
	if err != nil {
		return "", err
	}

	if rt.Jail != "" {
		if !IsInJail(rt.Jail, resolved) {
			return "", fmt.Errorf("ResolvePath outside of jail %s: %w", resolved, ErrEscapeAttempt)
		}
		return RemoveJailPrefix(rt.Jail, resolved), nil
	}

	return filepath.Clean(resolved), nil
}

// RelativePath returns path relative to basepath. If computation fails, target
// absolute path is returned.
func (rt *Runtime) RelativePath(basepath, path string) string {
	base, err := rt.AbsPath(basepath)
	if err != nil {
		base = basepath
	}
	target, err := rt.AbsPath(path)
	if err != nil {
		target = path
	}

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

func (rt *Runtime) ReadFile(rel string) ([]byte, error) {
	if err := rt.Validate(); err != nil {
		return nil, err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return nil, err
	}
	return rt.FS.ReadFile(rt.hostPath(path))
}

func (rt *Runtime) WriteFile(rel string, data []byte, perm os.FileMode) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	host := rt.hostPath(path)
	if err := rt.FS.Mkdir(filepath.Dir(host), 0o755, true); err != nil {
		return err
	}
	return rt.FS.WriteFile(host, data, perm)
}

func (rt *Runtime) Mkdir(rel string, perm os.FileMode, all bool) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	return rt.FS.Mkdir(rt.hostPath(path), perm, all)
}

func (rt *Runtime) Remove(rel string, all bool) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	return rt.FS.Remove(rt.hostPath(path), all)
}

func (rt *Runtime) Rename(src, dst string) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	srcPath, err := rt.ResolvePath(src, false)
	if err != nil {
		return err
	}
	dstPath, err := rt.ResolvePath(dst, false)
	if err != nil {
		return err
	}
	return rt.FS.Rename(rt.hostPath(srcPath), rt.hostPath(dstPath))
}

func (rt *Runtime) Stat(rel string, follow bool) (os.FileInfo, error) {
	if err := rt.Validate(); err != nil {
		return nil, err
	}
	path, err := rt.ResolvePath(rel, follow)
	if err != nil {
		return nil, err
	}
	return rt.FS.Stat(rt.hostPath(path), follow)
}

func (rt *Runtime) ReadDir(rel string) ([]os.DirEntry, error) {
	if err := rt.Validate(); err != nil {
		return nil, err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return nil, err
	}
	return rt.FS.ReadDir(rt.hostPath(path))
}

func (rt *Runtime) Symlink(oldname, newname string) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	oldPath, err := rt.ResolvePath(oldname, false)
	if err != nil {
		return err
	}
	newPath, err := rt.ResolvePath(newname, false)
	if err != nil {
		return err
	}
	return rt.FS.Symlink(rt.hostPath(oldPath), rt.hostPath(newPath))
}

func (rt *Runtime) Glob(pattern string) ([]string, error) {
	if err := rt.Validate(); err != nil {
		return nil, err
	}

	expanded, err := ExpandPath(rt.Env, pattern)
	if err != nil {
		return nil, err
	}

	wd, err := rt.Env.Getwd()
	if err != nil {
		return nil, err
	}

	virtualPattern := expanded
	if !filepath.IsAbs(virtualPattern) {
		virtualPattern = filepath.Join(wd, virtualPattern)
	}
	virtualPattern = filepath.Clean(virtualPattern)

	hostPattern := rt.hostPath(virtualPattern)
	matches, err := rt.FS.Glob(hostPattern)
	if err != nil {
		return nil, err
	}

	if rt.Jail == "" {
		return matches, nil
	}

	results := make([]string, 0, len(matches))
	for _, match := range matches {
		if !IsInJail(rt.Jail, match) {
			continue
		}
		jailedPath := RemoveJailPrefix(rt.Jail, match)
		if !filepath.IsAbs(expanded) {
			relPath, err := filepath.Rel(wd, jailedPath)
			if err == nil {
				results = append(results, relPath)
				continue
			}
		}
		results = append(results, jailedPath)
	}

	return results, nil
}

func (rt *Runtime) AtomicWriteFile(rel string, data []byte, perm os.FileMode) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	return rt.FS.AtomicWriteFile(rt.hostPath(path), data, perm)
}
